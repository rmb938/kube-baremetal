package api

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func HasFinalizer(obj metav1.Object, finalizer string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}

	return false
}

func RemoveFinalizer(obj metav1.Object, finalizer string) {
	finalizers := make([]string, 0)
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			continue
		}
		finalizers = append(finalizers, f)
	}
	obj.SetFinalizers(finalizers)
}
