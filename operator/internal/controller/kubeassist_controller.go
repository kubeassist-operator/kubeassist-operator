/*
Copyright 2025 KubeAssist Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeassistv1alpha1 "github.com/kubeassist-operator/kubeassist-operator/api/v1alpha1"
)

// clusterDomainCache caches the cluster domain resolved at startup.
var clusterDomainCache string

const (
	finalizerName = "kubeassist.github.io/finalizer"
	appLabel      = "kubeassist"
)

// KubeAssistReconciler reconciles a KubeAssist object.
type KubeAssistReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kubeassist.github.io,resources=kubeassists,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kubeassist.github.io,resources=kubeassists/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kubeassist.github.io,resources=kubeassists/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services;serviceaccounts;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings;rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop for KubeAssist CRs.
func (r *KubeAssistReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	startTime := time.Now()

	// Fetch the KubeAssist CR.
	agent := &kubeassistv1alpha1.KubeAssist{}
	if err := r.Get(ctx, req.NamespacedName, agent); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion via finalizer.
	if !agent.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(agent, finalizerName) {
			if err := r.cleanupClusterResources(ctx, agent); err != nil {
				logger.Error(err, "Failed to cleanup cluster-scoped resources")
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}
			controllerutil.RemoveFinalizer(agent, finalizerName)
			if err := r.Update(ctx, agent); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if missing.
	if !controllerutil.ContainsFinalizer(agent, finalizerName) {
		controllerutil.AddFinalizer(agent, finalizerName)
		if err := r.Update(ctx, agent); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Apply defaults.
	setDefaults(agent)

	// Reconcile owned resources in order.
	var reconcileErr error
	for _, fn := range []func(context.Context, *kubeassistv1alpha1.KubeAssist) error{
		r.reconcileServiceAccount,
		r.reconcileConfigMap,
		r.reconcileDeployment,
		r.reconcileService,
		r.reconcileExposure,
		r.reconcileClusterRBAC,
		r.reconcileNamespaceRBAC,
	} {
		if err := fn(ctx, agent); err != nil {
			reconcileErr = err
			break
		}
	}

	// Re-fetch before status update to get latest resourceVersion.
	if err := r.Get(ctx, req.NamespacedName, agent); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.updateStatus(ctx, agent, reconcileErr); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	_ = startTime // available for future metrics

	if reconcileErr != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeAssistReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeassistv1alpha1.KubeAssist{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

// ─────────────────────────────────────────────────────────────────────────────
// Defaults
// ─────────────────────────────────────────────────────────────────────────────

func setDefaults(h *kubeassistv1alpha1.KubeAssist) {
	if h.Spec.Image == "" {
		h.Spec.Image = "ghcr.io/kubeassist-operator/agent:latest"
	}
	if h.Spec.Mode == "" {
		h.Spec.Mode = "default"
	}
	if h.Spec.Replicas == nil {
		h.Spec.Replicas = ptr.To(int32(1))
	}
	if h.Spec.WorkspaceSize == "" {
		h.Spec.WorkspaceSize = "5Gi"
	}
	if h.Spec.RouteTimeout == "" {
		h.Spec.RouteTimeout = "600s"
	}
	if h.Spec.ApiKeySecretRef.Key == "" {
		h.Spec.ApiKeySecretRef.Key = "API_KEY"
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ServiceAccount
// ─────────────────────────────────────────────────────────────────────────────

func (r *KubeAssistReconciler) reconcileServiceAccount(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labels(h),
		},
	}
	if h.Spec.ImagePullSecretName != "" {
		sa.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: h.Spec.ImagePullSecretName},
		}
	}
	return r.createOrUpdate(ctx, h, sa)
}

// ─────────────────────────────────────────────────────────────────────────────
// ConfigMap (agent custom_modes.yaml)
// ─────────────────────────────────────────────────────────────────────────────

func (r *KubeAssistReconciler) reconcileConfigMap(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name + "-config",
			Namespace: h.Namespace,
			Labels:    labels(h),
		},
		Data: map[string]string{
			"agent_config.yaml": agentConfigYAML(h.Spec.Mode),
		},
	}
	return r.createOrUpdate(ctx, h, cm)
}

func agentConfigYAML(mode string) string {
	return fmt.Sprintf(`# KubeAssist agent configuration
# Generated by kubeassist-operator — do not edit manually.
agent:
  mode: %s
  maxConcurrentJobs: 100
  workdir: /workspace
  homeDir: /agent-home
`, mode)
}

// ─────────────────────────────────────────────────────────────────────────────
// Deployment
// ─────────────────────────────────────────────────────────────────────────────

func (r *KubeAssistReconciler) reconcileDeployment(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	wsSize := resource.MustParse(h.Spec.WorkspaceSize)
	homeSize := resource.MustParse("200Mi")

	// Hash config so pods restart when agent config changes.
	configHash := fmt.Sprintf("%x", sha256.Sum256([]byte(agentConfigYAML(h.Spec.Mode))))[:12]

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labels(h),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: h.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels(h),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels(h),
					Annotations: map[string]string{
						"kubeassist.github.io/config-hash": configHash,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: h.Name,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Name:            "agent",
						Image:           h.Spec.Image,
						ImagePullPolicy: corev1.PullAlways,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						}},
						Env: []corev1.EnvVar{
							{
								// Inject API key using the configurable key name.
								// Default key name is "API_KEY" — works with any provider.
								Name: "AGENT_API_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: h.Spec.ApiKeySecretRef.Name,
										},
										Key: h.Spec.ApiKeySecretRef.Key,
									},
								},
							},
							{Name: "AGENT_MODE", Value: h.Spec.Mode},
							{Name: "AGENT_WORKDIR", Value: "/workspace"},
							{Name: "AGENT_MAX_JOBS", Value: "100"},
							{Name: "HOME", Value: "/agent-home"},
							{Name: "PYTHONUNBUFFERED", Value: "1"},
						},
						Resources: h.Spec.Resources,
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/health",
									Port: intstr.FromInt(8080),
								},
							},
							InitialDelaySeconds: 20,
							PeriodSeconds:       10,
							FailureThreshold:    12,
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/health",
									Port: intstr.FromInt(8080),
								},
							},
							InitialDelaySeconds: 60,
							PeriodSeconds:       20,
							FailureThreshold:    5,
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
							ReadOnlyRootFilesystem:   ptr.To(false), // agent needs write access to workspace
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "workspace", MountPath: "/workspace"},
							{Name: "agent-home", MountPath: "/agent-home"},
							{Name: "agent-config", MountPath: "/agent-config"},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &wsSize},
							},
						},
						{
							Name: "agent-home",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &homeSize},
							},
						},
						{
							Name: "agent-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: h.Name + "-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if h.Spec.ImagePullSecretName != "" {
		dep.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: h.Spec.ImagePullSecretName},
		}
	}

	return r.createOrUpdate(ctx, h, dep)
}

// ─────────────────────────────────────────────────────────────────────────────
// Service
// ─────────────────────────────────────────────────────────────────────────────

func (r *KubeAssistReconciler) reconcileService(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labels(h),
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels(h),
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
				Protocol:   corev1.ProtocolTCP,
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	return r.createOrUpdate(ctx, h, svc)
}

// ─────────────────────────────────────────────────────────────────────────────
// Exposure — Ingress (vanilla k8s) + Route (OpenShift, best-effort)
// ─────────────────────────────────────────────────────────────────────────────

// reconcileExposure creates either a standard Ingress, an OpenShift Route, or
// both, depending on the spec and the cluster type.
//   - Ingress is created when spec.ingress.enabled=true (works everywhere)
//   - Route is attempted when spec.ingress.enabled=false (OCP clusters only);
//     on vanilla k8s the Route API is absent so the call is silently skipped
func (r *KubeAssistReconciler) reconcileExposure(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	if h.Spec.Ingress.Enabled {
		return r.reconcileIngress(ctx, h)
	}
	// Attempt OpenShift Route; ignore "no kind registered" errors on vanilla k8s
	return r.reconcileRoute(ctx, h)
}

func (r *KubeAssistReconciler) reconcileIngress(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        h.Name,
			Namespace:   h.Namespace,
			Labels:      labels(h),
			Annotations: h.Spec.Ingress.Annotations,
		},
	}

	pathType := networkingv1.PathTypePrefix
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ing, func() error {
		ing.Spec = networkingv1.IngressSpec{
			IngressClassName: h.Spec.Ingress.IngressClassName,
			Rules: []networkingv1.IngressRule{{
				Host: h.Spec.Ingress.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: h.Name,
									Port: networkingv1.ServiceBackendPort{Number: 8080},
								},
							},
						}},
					},
				},
			}},
		}
		if h.Spec.Ingress.TLSSecretName != "" {
			ing.Spec.TLS = []networkingv1.IngressTLS{{
				Hosts:      []string{h.Spec.Ingress.Host},
				SecretName: h.Spec.Ingress.TLSSecretName,
			}}
		}
		return ctrl.SetControllerReference(h, ing, r.Scheme)
	})
	return err
}

func (r *KubeAssistReconciler) reconcileRoute(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	})
	route.SetName(h.Name)
	route.SetNamespace(h.Namespace)
	route.SetLabels(labels(h))
	route.SetAnnotations(map[string]string{
		"haproxy.router.openshift.io/timeout": h.Spec.RouteTimeout,
	})
	route.Object["spec"] = map[string]interface{}{
		"to": map[string]interface{}{
			"kind": "Service",
			"name": h.Name,
		},
		"port": map[string]interface{}{
			"targetPort": "http",
		},
		"tls": map[string]interface{}{
			"termination":                   "edge",
			"insecureEdgeTerminationPolicy": "Redirect",
		},
	}

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(route.GroupVersionKind())
	key := types.NamespacedName{Name: h.Name, Namespace: h.Namespace}
	err := r.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		if setErr := ctrl.SetControllerReference(h, route, r.Scheme); setErr != nil {
			return setErr
		}
		createErr := r.Create(ctx, route)
		// Gracefully skip if Route API is not available (vanilla k8s)
		if createErr != nil && meta.IsNoMatchError(createErr) {
			return nil
		}
		return createErr
	}
	return err
}

// ─────────────────────────────────────────────────────────────────────────────
// Cluster RBAC
// ─────────────────────────────────────────────────────────────────────────────

func (r *KubeAssistReconciler) reconcileClusterRBAC(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	if err := r.ensureClusterRoleBinding(ctx, h, "view"); err != nil {
		return err
	}
	return r.ensureClusterRoleBinding(ctx, h, "cluster-reader")
}

func (r *KubeAssistReconciler) ensureClusterRoleBinding(ctx context.Context, h *kubeassistv1alpha1.KubeAssist, roleName string) error {
	name := fmt.Sprintf("%s-%s", h.Name, roleName)
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels(h),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      h.Name,
			Namespace: h.Namespace,
		}},
	}

	existing := &rbacv1.ClusterRoleBinding{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, crb)
	}
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepEqual(existing.Subjects, crb.Subjects) {
		existing.Subjects = crb.Subjects
		return r.Update(ctx, existing)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Namespace RBAC (RoleBindings for targetNamespaces)
// ─────────────────────────────────────────────────────────────────────────────

func (r *KubeAssistReconciler) reconcileNamespaceRBAC(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	for _, ns := range h.Spec.TargetNamespaces {
		if err := r.ensureRoleBinding(ctx, h, ns); err != nil {
			return err
		}
	}
	return nil
}

func (r *KubeAssistReconciler) ensureRoleBinding(ctx context.Context, h *kubeassistv1alpha1.KubeAssist, namespace string) error {
	name := fmt.Sprintf("%s-edit", h.Name)
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels(h),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "edit",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      h.Name,
			Namespace: h.Namespace,
		}},
	}

	existing := &rbacv1.RoleBinding{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, rb)
	}
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepEqual(existing.Subjects, rb.Subjects) {
		existing.Subjects = rb.Subjects
		return r.Update(ctx, existing)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Cleanup (finalizer)
// ─────────────────────────────────────────────────────────────────────────────

func (r *KubeAssistReconciler) cleanupClusterResources(ctx context.Context, h *kubeassistv1alpha1.KubeAssist) error {
	logger := log.FromContext(ctx)
	for _, role := range []string{"view", "cluster-reader"} {
		name := fmt.Sprintf("%s-%s", h.Name, role)
		crb := &rbacv1.ClusterRoleBinding{}
		if err := r.Get(ctx, types.NamespacedName{Name: name}, crb); err == nil {
			if err := r.Delete(ctx, crb); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			logger.Info("Deleted ClusterRoleBinding", "name", name)
		}
	}
	for _, ns := range h.Spec.TargetNamespaces {
		name := fmt.Sprintf("%s-edit", h.Name)
		rb := &rbacv1.RoleBinding{}
		if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, rb); err == nil {
			if err := r.Delete(ctx, rb); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			logger.Info("Deleted RoleBinding", "name", name, "namespace", ns)
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Status update
// ─────────────────────────────────────────────────────────────────────────────

func (r *KubeAssistReconciler) updateStatus(ctx context.Context, h *kubeassistv1alpha1.KubeAssist, reconcileErr error) error {
	dep := &appsv1.Deployment{}
	depKey := types.NamespacedName{Name: h.Name, Namespace: h.Namespace}

	phase := kubeassistv1alpha1.PhasePending
	availableCondition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		Reason:             "Progressing",
		Message:            "Deployment is being created",
		LastTransitionTime: metav1.Now(),
	}

	if reconcileErr != nil {
		phase = kubeassistv1alpha1.PhaseFailed
		availableCondition.Reason = "ReconcileError"
		availableCondition.Message = reconcileErr.Error()
	} else if err := r.Get(ctx, depKey, dep); err == nil {
		if dep.Status.ReadyReplicas > 0 && dep.Status.ReadyReplicas == *h.Spec.Replicas {
			phase = kubeassistv1alpha1.PhaseRunning
			availableCondition.Status = metav1.ConditionTrue
			availableCondition.Reason = "DeploymentReady"
			availableCondition.Message = fmt.Sprintf("%d/%d replicas ready", dep.Status.ReadyReplicas, *h.Spec.Replicas)
		} else {
			availableCondition.Reason = "Progressing"
			availableCondition.Message = fmt.Sprintf("%d/%d replicas ready", dep.Status.ReadyReplicas, *h.Spec.Replicas)
		}
	}

	h.Status.Phase = phase
	h.Status.ObservedGeneration = h.Generation
	meta.SetStatusCondition(&h.Status.Conditions, availableCondition)

	// Populate URL — prefer Ingress host, fall back to Route convention
	if h.Spec.Ingress.Enabled && h.Spec.Ingress.Host != "" {
		scheme := "http"
		if h.Spec.Ingress.TLSSecretName != "" {
			scheme = "https"
		}
		h.Status.URL = fmt.Sprintf("%s://%s", scheme, h.Spec.Ingress.Host)
	} else {
		h.Status.URL = fmt.Sprintf("https://%s-%s.apps.%s", h.Name, h.Namespace, getClusterDomain())
	}

	return r.Status().Update(ctx, h)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func labels(h *kubeassistv1alpha1.KubeAssist) map[string]string {
	return map[string]string{
		"app":                          appLabel,
		"app.kubernetes.io/name":       h.Name,
		"app.kubernetes.io/managed-by": "kubeassist-operator",
		"app.kubernetes.io/instance":   h.Name,
		"app.kubernetes.io/part-of":    "kubeassist",
	}
}

func selectorLabels(h *kubeassistv1alpha1.KubeAssist) map[string]string {
	return map[string]string{
		"app":                    appLabel,
		"app.kubernetes.io/name": h.Name,
	}
}

// getClusterDomain returns the cluster base domain for Route URL construction.
// Falls back to "cluster.local" on non-OCP clusters.
func getClusterDomain() string {
	if clusterDomainCache != "" {
		return clusterDomainCache
	}
	return "cluster.local"
}

// ResolveClusterDomain probes the OpenShift ingresses config to get the real
// cluster domain. Called once at startup in a goroutine.
func ResolveClusterDomain(c client.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ingress := &unstructured.Unstructured{}
	ingress.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Version: "v1",
		Kind:    "Ingress",
	})
	if err := c.Get(ctx, types.NamespacedName{Name: "cluster"}, ingress); err != nil {
		return // not an OCP cluster or permission denied — use fallback
	}
	domain, _, _ := unstructured.NestedString(ingress.Object, "spec", "domain")
	if domain != "" {
		clusterDomainCache = domain
	}
}

// createOrUpdate is a typed helper that sets owner reference and calls
// controllerutil.CreateOrUpdate.
func (r *KubeAssistReconciler) createOrUpdate(ctx context.Context, h *kubeassistv1alpha1.KubeAssist, obj client.Object) error {
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		return ctrl.SetControllerReference(h, obj, r.Scheme)
	})
	return err
}
