# Tool to enable ZIU from 2011 to 21.x/master

2011 version has different laytou of services (how services are mapped 
between k8s pods). So ZIU update logic cannot work as is and operator crashes.

ZIU update from 2011 to 21.x/master workflow

1. Stop operator to avoid side-effects
```bash
kubectl -n tf delete deployment tf-operator
```

2. Update CRDs
```bash
kubectl apply -f ~/tf-operator/deploy/crds/
```

3. Update manager manifest
```bash
kubectl apply -k ~/tf-operator/deploy/kustomize/contrail/templates/
```

4. Run cli tool to hack 2011 objects

4.1. If the tool is not built - build it.
(If golang is not installed - install it as in [main readme](../../Readme.md) )
```bash
cd ~/tf-operator/contrib/ziu-2011-to-21.4-hack
go build
```

4.2. Run the tool
```bash 
~/tf-operator/contrib/ziu-2011-to-21.4-hack/ziu-2011-to-21.4-hack --repeate 180 --delay 10
```
Note that real values depends on the setup, usually ZIU takes 15-30 minutes.

5. Start new operator that runs ZIU procedure
```bash
kubectl apply -k ~/tf-operator/deploy/kustomize/operator/templates/
```

6. Wait till ZIU completed and cancel the tool if ZIU finished earlier
