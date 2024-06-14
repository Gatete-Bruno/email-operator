package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	emailv1alpha1 "github.com/Gatete-Bruno/mailsend-k8s-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"io/ioutil"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EmailReconciler reconciles an Email object
type EmailReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=email.batman.example.com,resources=emails,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=email.batman.example.com,resources=emails/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=email.batman.example.com,resources=emails/finalizers,verbs=update

func (r *EmailReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("email", req.NamespacedName)

	// Fetch the Email instance
	var email emailv1alpha1.Email
	if err := r.Get(ctx, req.NamespacedName, &email); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Fetch the EmailSenderConfig referenced by this email
	log.Info("Fetching EmailSenderConfig", "name", email.Spec.SenderConfigRef)
	var config emailv1alpha1.EmailSenderConfig
	if err := r.Get(ctx, types.NamespacedName{Name: email.Spec.SenderConfigRef, Namespace: req.Namespace}, &config); err != nil {
		log.Error(err, "Failed to fetch EmailSenderConfig", "name", email.Spec.SenderConfigRef)
		return ctrl.Result{}, err
	}
	log.Info("Successfully fetched EmailSenderConfig", "config", config)

	// Fetch the API token from the referenced secret
	var secret v1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: config.Spec.ApiTokenSecretRef, Namespace: req.Namespace}, &secret); err != nil {
		return ctrl.Result{}, err
	}

	apiToken, ok := secret.Data["api-token"]
	if !ok {
		return ctrl.Result{}, fmt.Errorf("apiToken not found in secret %s", config.Spec.ApiTokenSecretRef)
	}

	// Send the email using MailerSend API
	err := r.sendEmailWithMailersend(ctx, email, config, apiToken)
	if err != nil {
		log.Error(err, "Failed to send email using MailerSend")
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled email", "email", req.NamespacedName)
	return ctrl.Result{}, nil
}

// sendEmailWithMailersend sends email using the MailerSend API
func (r *EmailReconciler) sendEmailWithMailersend(ctx context.Context, email emailv1alpha1.Email, config emailv1alpha1.EmailSenderConfig, apiToken []byte) error {
	// Create an HTTP client with SSL verification enabled
	client := &http.Client{}

	reqBody, err := json.Marshal(map[string]interface{}{
		"from": map[string]string{
			"email": config.Spec.SenderEmail,
		},
		"to": []map[string]string{
			{
				"email": email.Spec.RecipientEmail,
			},
		},
		"subject": email.Spec.Subject,
		"text":    email.Spec.Body,
	})
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", "https://api.mailersend.com/v1/email", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	request.Header.Set("Authorization", "Bearer "+string(apiToken))
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(body, &respData); err != nil {
		return err
	}

	// Update the status of the email resource based on the response
	if response.StatusCode == http.StatusOK {
		email.Status.DeliveryStatus = "Sent"
		email.Status.MessageID = respData["message_id"].(string)
	} else {
		email.Status.DeliveryStatus = "Failed"
		if errMsg, ok := respData["error"].(string); ok {
			email.Status.Error = errMsg
		} else {
			email.Status.Error = "Unknown error"
		}
	}

	if err := r.Status().Update(ctx, &email); err != nil {
		return err
	}

	return nil
}

func (r *EmailReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&emailv1alpha1.Email{}).
		Complete(r)
}
