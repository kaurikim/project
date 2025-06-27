```bash
[kauri@kdev project]$ make deploy IMG=pcvm:8282/project:latest
/home/kauri/project/bin/controller-gen rbac:roleName=manager-role crd:maxDescLen=0 webhook paths="./..." output:crd:artifacts:config=config/crd/bases
cd config/manager && /home/kauri/project/bin/kustomize edit set image controller=pcvm:8282/project:latest
/home/kauri/project/bin/kustomize build config/default | kubectl apply -f -
namespace/project-system created
customresourcedefinition.apiextensions.k8s.io/cronjobs.batch.tutorial.kubebuilder.io unchanged
serviceaccount/project-controller-manager created
role.rbac.authorization.k8s.io/project-leader-election-role created
clusterrole.rbac.authorization.k8s.io/project-cronjob-admin-role created
clusterrole.rbac.authorization.k8s.io/project-cronjob-editor-role created
clusterrole.rbac.authorization.k8s.io/project-cronjob-viewer-role created
clusterrole.rbac.authorization.k8s.io/project-manager-role created
clusterrole.rbac.authorization.k8s.io/project-metrics-auth-role created
clusterrole.rbac.authorization.k8s.io/project-metrics-reader created
rolebinding.rbac.authorization.k8s.io/project-leader-election-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/project-manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/project-metrics-auth-rolebinding created
service/project-controller-manager-metrics-service created
service/project-webhook-service created
deployment.apps/project-controller-manager created
mutatingwebhookconfiguration.admissionregistration.k8s.io/project-mutating-webhook-configuration created
validatingwebhookconfiguration.admissionregistration.k8s.io/project-validating-webhook-configuration created
[kauri@kdev project]$ 
```