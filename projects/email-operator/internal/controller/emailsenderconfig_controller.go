package controllers

import (
	"context"

	emailv1alpha1 "github.com/Gatete-Bruno/mailsend-k8s-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// EmailSenderConfigReconciler reconciles an EmailSenderConfig object
type EmailSenderConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=email.batman.example.com,resources=emailsenderconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=email.batman.example.com,resources=emailsenderconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=email.batman.example.com,resources=emailsenderconfigs/finalizers,verbs=update

func (r *EmailSenderConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("emailsenderconfig", req.NamespacedName)
	log.Info("Reconciling EmailSenderConfig")

	// Fetch the EmailSenderConfig instance
	var config emailv1alpha1.EmailSenderConfig
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: "email-operator-system"}, &config); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(&config, "emailSenderConfig.finalizers.batman.example.com") {
		controllerutil.AddFinalizer(&config, "emailSenderConfig.finalizers.batman.example.com")
		if err := r.Update(ctx, &config); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if the EmailSenderConfig instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if !config.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&config, "emailSenderConfig.finalizers.batman.example.com") {
			// Perform finalization logic for emailSenderConfigFinalizer
			log.Info("Performing Finalizer Operations for EmailSenderConfig")

			// Remove finalizer once finalization is done
			controllerutil.RemoveFinalizer(&config, "emailSenderConfig.finalizers.batman.example.com")
			if err := r.Update(ctx, &config); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Log the creation or update of the EmailSenderConfig
	if config.CreationTimestamp == config.Status.LastUpdateTime {
		log.Info("EmailSenderConfig created", "EmailSenderConfig", req.NamespacedName)
	} else {
		log.Info("EmailSenderConfig updated", "EmailSenderConfig", req.NamespacedName)
	}

	// Update the status of the EmailSenderConfig
	config.Status.LastUpdateTime = config.CreationTimestamp
	if err := r.Status().Update(ctx, &config); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EmailSenderConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&emailv1alpha1.EmailSenderConfig{}).
		Complete(r)
}
