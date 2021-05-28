package controllers

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8sv1alpha1 "github.com/basilfx/templated-secrets/api/v1alpha1"
)

// TemplateRegex can be used to parse variable references (e.g. `$(..)`)
var TemplateRegex = regexp.MustCompile(`(?m)\$\([-_\.a-zA-Z0-9]+(?:\s*>\s*[-_\.a-zA-Z0-9]+)+\)`)

// TemplatedSecretReconciler reconciles a TemplatedSecret object
type TemplatedSecretReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type Variable struct {
	Namespace string
	SecretRef string
	Key       string
	Value     string
}

// +kubebuilder:rbac:groups=k8s.basilfx.net,resources=templatedsecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.basilfx.net,resources=templatedsecrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.basilfx.net,resources=templatedsecrets/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the TemplatedSecret object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *TemplatedSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("templatedsecret", req.NamespacedName)

	// Fetch the TemplatedSecret instance.
	ts := &k8sv1alpha1.TemplatedSecret{}
	err := r.Get(ctx, req.NamespacedName, ts)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("TemplatedSecret resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to retrieve TemplatedSecret.")
		return ctrl.Result{}, err
	}

	// Inspect the template for variables.
	variables := make(map[string]*Variable)

	for k, v := range ts.Spec.Data {
		log.V(1).Info("Parsing template.", "key", k, "value", v)

		for _, match := range TemplateRegex.FindAllString(v, -1) {
			log.V(1).Info("Found match", "match", match)

			variables[match] = r.parseVariable(ts, match)
		}
	}

	// For every variable, lookup the value.
	for k, v := range variables {
		s := &v1.Secret{}
		err := r.Get(ctx, types.NamespacedName{Namespace: v.Namespace, Name: v.SecretRef}, s)

		// If the secret is not found, then update the status and requeue.
		if err != nil && errors.IsNotFound(err) {
			log.Info("Referenced secret not found", "variable", v)

			ts.Status.Message = fmt.Sprintf("Unable to resolve variable '%s' because the secret does not exist.", k)
			err := r.Status().Update(ctx, ts)

			if err != nil {
				log.Error(err, "Failed to update TemplatedSecret status.")
				return ctrl.Result{}, err
			}

			return ctrl.Result{Requeue: true}, nil
		}

		// Another error ocurred, fail here.
		if err != nil {
			log.Error(err, "Failed to retrieve referenced Secret.", "variable", v)
			return ctrl.Result{}, err
		}

		// Resolve the value.
		raw, ok := s.Data[v.Key]

		if !ok {
			log.Info("Key in secret not found.", "variable", v)

			ts.Status.Message = fmt.Sprintf("Unable to resolve variable '%s' because the key '%s' was not found in secret '%s'.", k, v.Key, v.SecretRef)
			err := r.Status().Update(ctx, ts)

			if err != nil {
				log.Error(err, "Failed to update TemplatedSecret status.")
				return ctrl.Result{}, err
			}

			return ctrl.Result{Requeue: true}, nil
		}

		variables[k].Value = string(raw)
	}

	// Find existing secret.
	s := &v1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Namespace: ts.Namespace, Name: ts.Name}, s)

	if err != nil {
		if errors.IsNotFound(err) {
			s = r.createSecret(ts)
		} else {
			log.Error(err, "Failed to retrieve Secret.")
			return ctrl.Result{}, err
		}
	}

	// Check if the (existings) secret is owned by us.
	owned := false

	for _, v := range s.OwnerReferences {
		if v.UID == ts.UID {
			owned = true
			break
		}
	}

	if !owned {
		log.Info("Existing Secret not owned by TemplatedSecret.", "secret", fmt.Sprintf("%s/%s", s.Namespace, s.Name))

		ts.Status.Message = fmt.Sprintf("Secret '%s' is not owned by TemplatedSecret '%s'. Not updating.", fmt.Sprintf("%s/%s", s.Namespace, s.Name), fmt.Sprintf("%s/%s", ts.Namespace, ts.Name))
		err = r.Status().Update(ctx, ts)

		if err != nil {
			log.Error(err, "Failed to update status of TemplatedSecret.")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, err
	}

	// Update secret from scratch, replace all variable references with their
	// resolved values.
	s.Data = make(map[string][]byte)

	for k, v := range ts.Spec.Data {
		for kk, vv := range variables {
			v = strings.Replace(v, kk, vv.Value, 1)
		}

		s.Data[k] = []byte(v)
	}

	if s.ObjectMeta.UID == "" {
		log.V(1).Info("Creating new secret", "secret", fmt.Sprintf("%s/%s", s.Namespace, s.Name))

		err := r.Create(ctx, s)

		if err != nil {
			log.Error(err, "Failed to create new Secret.", "secret", fmt.Sprintf("%s/%s", s.Namespace, s.Name))
			return ctrl.Result{}, err
		}
	} else {
		log.V(1).Info("Updating existing secret", "secret", fmt.Sprintf("%s/%s", s.Namespace, s.Name))

		err := r.Update(ctx, s)

		if err != nil {
			log.Error(err, "Failed to update existing Secret.", "secret", fmt.Sprintf("%s/%s", s.Namespace, s.Name))
			return ctrl.Result{}, err
		}
	}

	// Update status of TemplatedSecret.
	ts.Status.Message = fmt.Sprintf("Secret '%s' is up to date.", fmt.Sprintf("%s/%s", s.Namespace, s.Name))
	err = r.Status().Update(ctx, ts)

	if err != nil {
		log.Error(err, "Failed to update status of TemplatedSecret.")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// parseVariable parses a match and returns a Variable object. The match must
// be `$(namespace > secretRef > key)` or `$(secretRef > key)`.
func (r *TemplatedSecretReconciler) parseVariable(ts *k8sv1alpha1.TemplatedSecret, match string) *Variable {
	parts := strings.Split(match[2:len(match)-1], ">")

	if len(parts) == 2 {
		return &Variable{
			Namespace: ts.Namespace,
			SecretRef: strings.TrimSpace(parts[0]),
			Key:       strings.TrimSpace(parts[1]),
		}
	} else if len(parts) == 3 {
		return &Variable{
			Namespace: strings.TrimSpace(parts[0]),
			SecretRef: strings.TrimSpace(parts[1]),
			Key:       strings.TrimSpace(parts[2]),
		}
	} else {
		// Incomplete variable.
		return nil
	}
}

func (r *TemplatedSecretReconciler) createSecret(ts *k8sv1alpha1.TemplatedSecret) *v1.Secret {
	s := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ts.Spec.Template.ObjectMeta.Name,
			Labels:      ts.Spec.Template.ObjectMeta.Labels,
			Annotations: ts.Spec.Template.ObjectMeta.Annotations,
		},
		Type: ts.Spec.Template.Type,
	}

	// Use defaults for namespace and name if not set.
	if s.ObjectMeta.Namespace == "" {
		s.ObjectMeta.Namespace = ts.ObjectMeta.Namespace
	}

	if s.ObjectMeta.Name == "" {
		s.ObjectMeta.Name = ts.ObjectMeta.Name
	}

	// Link secret to the template.
	err := ctrl.SetControllerReference(ts, s, r.Scheme)

	if err != nil {
		panic(err)
	}

	return s
}

// SetupWithManager sets up the controller with the Manager.
func (r *TemplatedSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.TemplatedSecret{}).
		Owns(&v1.Secret{}).
		Complete(r)
}
