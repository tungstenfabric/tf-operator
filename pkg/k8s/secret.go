package k8s

import (
	"context"
	"fmt"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Secret struct {
	name      string
	ownerType string
	owner     v1.Object
	scheme    *runtime.Scheme
	client    client.Client
	Secret    *core.Secret
}

type SecretFiller interface {
	FillSecret(sc *core.Secret, force bool) error
}

func (s *Secret) EnsureExists(dataSetter SecretFiller, force bool) error {
	secret, err := s.createNewOrGetExistingSecret()
	if err != nil {
		return fmt.Errorf("Failed to create or get secret: %w", err)
	}
	if err = dataSetter.FillSecret(secret, force); err != nil {
		return err
	}
	if err = s.client.Update(context.Background(), secret); err != nil {
		return fmt.Errorf("Failed to update secret: %w", err)
	}
	s.Secret = secret
	return nil
}

func (s *Secret) createNewOrGetExistingSecret() (*core.Secret, error) {
	secret := &core.Secret{}
	namespacedName := types.NamespacedName{Name: s.name, Namespace: s.owner.GetNamespace()}
	err := s.client.Get(context.Background(), namespacedName, secret)

	if err == nil {
		return secret, nil
	}

	if errors.IsNotFound(err) {
		secret = &core.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      s.name,
				Namespace: s.owner.GetNamespace(),
				Labels: map[string]string{
					"tf_manager": s.ownerType,
					s.ownerType:  s.owner.GetName(),
				},
			},
			Data: make(map[string][]byte),
		}
		if err = controllerutil.SetControllerReference(s.owner, secret, s.scheme); err != nil {
			return nil, err
		}
		err = s.client.Create(context.Background(), secret)
		return secret, err
	}
	return nil, err
}
