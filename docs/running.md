# Running and deploying the controller

### Optional

API 정의에 변경을 가하고자 한다면, 진행하기 전에 다음 명령어로 CR 또는 CRD와 같은 매니페스트를 생성하세요:

```bash
make manifests
```

컨트롤러를 테스트하려면, 클러스터에 대해 로컬에서 실행할 수 있습니다. 그러나 실행하기 전에 [quick start](/quick-start)에 따라 CRD를 설치해야 합니다. 필요하다면 controller-tools를 사용해 YAML 매니페스트를 자동으로 업데이트할 것입니다:

```bash
make install
```

이제 CRD를 설치했으므로, 컨트롤러를 클러스터에 대해 실행할 수 있습니다. 이 명령은 현재 클러스터에 연결된 자격 증명을 사용하므로, 아직 RBAC에 대해 걱정할 필요가 없습니다.

# Running webhooks locally

웹훅을 로컬에서 실행하려면, 인증서를 생성하고 기본 디렉터리인 `/tmp/k8s-webhook-server/serving-certs/tls.{crt,key}`에 배치해야 합니다.

로컬 API 서버를 실행하지 않는 경우, 원격 클러스터에서 로컬 웹훅 서버로 트래픽을 프록시하도록 설정해야 합니다. 이러한 이유로, 아래와 같이 로컬 코드-실행-테스트 주기에서는 일반적으로 웹훅을 비활성화할 것을 권장합니다.

다른 터미널에서 다음을 실행하세요:

```bash
export ENABLE_WEBHOOKS=false
make run
```

컨트롤러가 시작 로그를 출력하지만, 아직 아무 작업도 수행하지 않을 것입니다.

이 시점에서 테스트용 CronJob이 필요합니다. `config/samples/batch_v1_cronjob.yaml`에 샘플을 작성하고 이를 사용해 보겠습니다:

```yaml
apiVersion: batch.tutorial.kubebuilder.io/v1
kind: CronJob
metadata:
  labels:
    app.kubernetes.io/name: project
    app.kubernetes.io/managed-by: kustomize
  name: cronjob-sample
spec:
  schedule: "*/1 * * * *"
  startingDeadlineSeconds: 60
  concurrencyPolicy: Allow # 명시적으로 지정하지만 Allow가 기본값이기도 합니다.
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: busybox
            args:
            - /bin/sh
            - -c
            - date; echo Hello from the Kubernetes cluster
          restartPolicy: OnFailure
```

```bash
kubectl create -f config/samples/batch_v1_cronjob.yaml
```

이 시점에 여러 작업이 진행되는 것을 확인할 수 있습니다. 변경 사항을 관찰하면, CronJob이 실행되고 상태가 업데이트되는 것을 볼 수 있을 것입니다:

```bash
kubectl get cronjob.batch.tutorial.kubebuilder.io -o yaml
kubectl get job
```

정상적으로 실행되는 것을 확인했으므로, 클러스터에서 실행해 보겠습니다. `make run` 실행을 중지한 후 다음을 실행하세요:

```bash
make docker-build docker-push IMG=<some-registry>/<project-name>:tag
make deploy IMG=<some-registry>/<project-name>:tag
```

# Registry Permission

이 이미지는 사용자가 지정한 개인 레지스트리에 게시되어야 합니다. 위 명령어가 동작하지 않을 경우, 해당 레지스트리에 대한 접근 권한이 있는지 확인하세요.

로컬 개발 및 CI 환경에서 더 빠르고 효율적인 워크플로우를 위해 Kind를 통합하는 것을 고려해 보세요. Kind 클러스터를 사용하는 경우, 원격 컨테이너 레지스트리에 이미지를 푸시할 필요가 없습니다. 로컬 이미지를 지정한 Kind 클러스터로 직접 로드할 수 있습니다:

```bash
kind load docker-image <your-image-name>:tag --name <your-kind-cluster-name>
```

자세한 내용은 [Using Kind For Development Purposes and CI](/reference/kind)를 참조하세요.

# RBAC errors

RBAC 오류가 발생하면, `cluster-admin` 권한을 부여하거나 관리자 계정으로 로그인해야 할 수 있습니다. [GKE 클러스터 v1.11.x 및 이전 버전의 Kubernetes RBAC 사용을 위한 사전 요구 사항](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control#iam-rolebinding-bootstrap)을 참조하세요.

컨트롤러가 다시 작동하는지 확인하기 위해 이전과 같이 CronJob 목록을 다시 조회해 보세요!
