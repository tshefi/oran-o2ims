/*
Copyright 2023.

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

package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	oranv1alpha1 "github.com/openshift-kni/oran-o2ims/api/v1alpha1"
	"github.com/openshift-kni/oran-o2ims/internal/controllers/utils"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var suitescheme = scheme.Scheme

func getFakeClientFromObjects(objs ...client.Object) (client.WithWatch, error) {
	return fake.NewClientBuilder().WithScheme(suitescheme).WithObjects(objs...).WithStatusSubresource(&oranv1alpha1.ORANO2IMS{}).Build(), nil
}

func TestORANO2IMSReconciler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("Bootstrapping test environment")

	suitescheme.AddKnownTypes(oranv1alpha1.GroupVersion, &oranv1alpha1.ORANO2IMS{})
	suitescheme.AddKnownTypes(oranv1alpha1.GroupVersion, &oranv1alpha1.ORANO2IMSList{})
	suitescheme.AddKnownTypes(networkingv1.SchemeGroupVersion, &networkingv1.Ingress{})
	suitescheme.AddKnownTypes(networkingv1.SchemeGroupVersion, &networkingv1.IngressList{})
	suitescheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.ServiceAccount{})
	suitescheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.ServiceAccountList{})
	suitescheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Service{})
	suitescheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.ServiceList{})
	suitescheme.AddKnownTypes(appsv1.SchemeGroupVersion, &appsv1.Deployment{})
	suitescheme.AddKnownTypes(appsv1.SchemeGroupVersion, &appsv1.DeploymentList{})
})

var _ = DescribeTable(
	"Reconciler",
	func(objs []client.Object, request reconcile.Request, validate func(result ctrl.Result, reconciler ORANO2IMSReconciler)) {
		// Declare the Namespace for the O-RAN O2IMS resource.
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "oran-o2ims",
			},
		}

		// Update the testcase objects to include the Namespace.
		objs = append(objs, ns)

		// Get the fake client.
		fakeClient, err := getFakeClientFromObjects(objs...)
		Expect(err).ToNot(HaveOccurred())

		// Initialize the O-RAN O2IMS reconciler.
		r := &ORANO2IMSReconciler{
			Client: fakeClient,
			Log:    logr.Discard(),
			Scheme: fakeClient.Scheme(),
		}

		// Reconcile.
		result, err := r.Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())

		validate(result, *r)
	},
	Entry(
		"Metadata server deployment is updated after edit",
		[]client.Object{
			&oranv1alpha1.ORANO2IMS{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "oran-o2ims-sample-1",
					Namespace:         utils.ORANO2IMSNamespace,
					CreationTimestamp: metav1.Now(),
				},
				Spec: oranv1alpha1.ORANO2IMSSpec{
					MetadataServer:          true,
					DeploymentManagerServer: false,
				},
			},
		},
		reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: utils.ORANO2IMSNamespace,
				Name:      "oran-o2ims-sample-1",
			},
		},
		func(result ctrl.Result, reconciler ORANO2IMSReconciler) {
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: 5 * time.Minute}))

			// Check that the metadata server deployment exists.
			metadataDeployment := &appsv1.Deployment{}
			err := reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSMetadataServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				metadataDeployment)
			Expect(err).ToNot(HaveOccurred())

			// Update one of the deployment's Spec values to something random.
			savedSpecTemplateVolumeSecret := metadataDeployment.Spec.Template.Spec.Volumes[0].Secret.SecretName
			savedContainersArgsValue := metadataDeployment.Spec.Template.Spec.Containers[0].Args
			metadataDeployment.Spec.Template.Spec.Volumes[0].Secret.SecretName = "made-up-name"
			metadataDeployment.Spec.Template.Spec.Containers[0].Args = []string{"a", "b"}

			// Run the reconciliation again.
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: utils.ORANO2IMSNamespace,
					Name:      "oran-o2ims-sample-1",
				},
			}
			_, err = reconciler.Reconcile(context.TODO(), req)
			Expect(err).ToNot(HaveOccurred())

			// Check that the fields edited above were restored to their previous value.
			err = reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSMetadataServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				metadataDeployment)
			Expect(err).ToNot(HaveOccurred())
			Expect(metadataDeployment.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal(savedSpecTemplateVolumeSecret))
			Expect(metadataDeployment.Spec.Template.Spec.Containers[0].Args).To(Equal(savedContainersArgsValue))
		},
	),
	Entry(
		"Only the metadata server is required",
		[]client.Object{
			&oranv1alpha1.ORANO2IMS{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "oran-o2ims-sample-1",
					Namespace:         utils.ORANO2IMSNamespace,
					CreationTimestamp: metav1.Now(),
				},
				Spec: oranv1alpha1.ORANO2IMSSpec{
					MetadataServer:          true,
					DeploymentManagerServer: false,
				},
			},
		},
		reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: utils.ORANO2IMSNamespace,
				Name:      "oran-o2ims-sample-1",
			},
		},
		func(result ctrl.Result, reconciler ORANO2IMSReconciler) {
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: 5 * time.Minute}))

			// Check the metadata server deployment exists.
			metadataDeployment := &appsv1.Deployment{}
			err := reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSMetadataServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				metadataDeployment)
			Expect(err).ToNot(HaveOccurred())

			// Check that the Ingress exists.
			ingress := &networkingv1.Ingress{}
			err = reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSIngressName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				ingress)
			Expect(err).ToNot(HaveOccurred())

			// Check that the ServiceAccount exists.
			serviceAccount := &corev1.ServiceAccount{}
			err = reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSMetadataServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				serviceAccount)
			Expect(err).ToNot(HaveOccurred())

			// Check that the Service exists.
			service := &corev1.Service{}
			err = reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSMetadataServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				service)
			Expect(err).ToNot(HaveOccurred())

			// Check the deployment manager server does not exist.
			deploymentManagerDeployment := &appsv1.Deployment{}
			err = reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSDeploymentManagerServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				deploymentManagerDeployment)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("deployments.apps \"%s\" not found", utils.ORANO2IMSDeploymentManagerServerName)))
		},
	),
	Entry(
		"Metadata and deployment manager servers required",
		[]client.Object{
			&oranv1alpha1.ORANO2IMS{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "oran-o2ims-sample-1",
					Namespace:         "oran-o2ims",
					CreationTimestamp: metav1.Now(),
				},
				Spec: oranv1alpha1.ORANO2IMSSpec{
					MetadataServer:          true,
					DeploymentManagerServer: true,
				},
			},
		},
		reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: utils.ORANO2IMSNamespace,
				Name:      "oran-o2ims-sample-1",
			},
		},
		func(result ctrl.Result, reconciler ORANO2IMSReconciler) {
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: 5 * time.Minute}))

			// Check that the metadata deployment exists.
			metadataDeployment := &appsv1.Deployment{}
			err := reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSMetadataServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				metadataDeployment)
			Expect(err).ToNot(HaveOccurred())

			// Check that the deployment manager server exists.
			deploymentManagerDeployment := &appsv1.Deployment{}
			err = reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSDeploymentManagerServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				deploymentManagerDeployment)
			Expect(err).ToNot(HaveOccurred())
		},
	),
	Entry(
		"No O-RAN O2IMS server required",
		[]client.Object{
			&oranv1alpha1.ORANO2IMS{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "oran-o2ims-sample-1",
					Namespace:         "oran-o2ims",
					CreationTimestamp: metav1.Now(),
				},
				Spec: oranv1alpha1.ORANO2IMSSpec{
					MetadataServer:          false,
					DeploymentManagerServer: false,
				},
			},
		},
		reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: utils.ORANO2IMSNamespace,
				Name:      "oran-o2ims-sample-1",
			},
		},
		func(result ctrl.Result, reconciler ORANO2IMSReconciler) {
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: 5 * time.Minute}))
			// Check the metadata server deployment does not exist.
			metadataDeployment := &appsv1.Deployment{}
			err := reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSMetadataServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				metadataDeployment)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("deployments.apps \"%s\" not found", utils.ORANO2IMSMetadataServerName)))

			// Check the deployment manager server does not exist.
			deploymentManagerDeployment := &appsv1.Deployment{}
			err = reconciler.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      utils.ORANO2IMSDeploymentManagerServerName,
					Namespace: utils.ORANO2IMSNamespace,
				},
				deploymentManagerDeployment)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Sprintf("deployments.apps \"%s\" not found", utils.ORANO2IMSDeploymentManagerServerName)))
		},
	),
)
