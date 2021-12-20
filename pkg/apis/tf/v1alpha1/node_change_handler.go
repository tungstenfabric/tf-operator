package v1alpha1

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NodeWatcher interface {
	GetEmptyListObject() runtime.Object
	GetItems(list interface{}) []types.NamespacedName
}

func NodeChangeHandler(watcher NodeWatcher, cl client.Client) handler.Funcs {
	return handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			list := watcher.GetEmptyListObject()
			err := cl.List(context.TODO(), list)
			if err == nil {
				for _, m := range watcher.GetItems(list) {
					q.Add(reconcile.Request{NamespacedName: m})
				}
			}
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			list := watcher.GetEmptyListObject()
			err := cl.List(context.TODO(), list)
			if err == nil {
				for _, m := range watcher.GetItems(list) {
					q.Add(reconcile.Request{NamespacedName: m})
				}
			}
		},
		DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
			list := watcher.GetEmptyListObject()
			err := cl.List(context.TODO(), list)
			if err == nil {
				for _, m := range watcher.GetItems(list) {
					q.Add(reconcile.Request{NamespacedName: m})
				}
			}
		},
		GenericFunc: func(e event.GenericEvent, q workqueue.RateLimitingInterface) {
			list := watcher.GetEmptyListObject()
			err := cl.List(context.TODO(), list)
			if err == nil {
				for _, m := range watcher.GetItems(list) {
					q.Add(reconcile.Request{NamespacedName: m})
				}
			}
		},
	}
}
