package vrouter

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
)

type nodeWatcher struct{}

func (*nodeWatcher) GetEmptyListObject() runtime.Object {
	return &v1alpha1.VrouterList{}
}

func (*nodeWatcher) GetItems(list interface{}) []types.NamespacedName {
	res := []types.NamespacedName{}
	for _, i := range list.(*v1alpha1.VrouterList).Items {
		res = append(res, types.NamespacedName{Name: i.GetName(), Namespace: i.GetNamespace()})
	}
	return res
}

func nodeChangeHandler(cl client.Client) handler.Funcs {
	return v1alpha1.NodeChangeHandler(&nodeWatcher{}, cl)
}
