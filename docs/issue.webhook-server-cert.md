첨부된 에러 로그를 분석해보겠습니다. 에러의 원인을 파악하기 위해 관련 파일들을 살펴보겠습니다.
Ran tool
Ran tool
Ran tool
Ran tool
Ran tool
Read file: docs/book/src/cronjob-tutorial/testdata/project/config/default/manager_webhook_patch.yaml
Read file: docs/book/src/cronjob-tutorial/testdata/project/config/certmanager/certificate-webhook.yaml
Read file: docs/book/src/cronjob-tutorial/testdata/project/config/certmanager/kustomization.yaml
Ran tool
Read file: docs/book/src/cronjob-tutorial/testdata/project/config/certmanager/issuer.yaml
Ran tool

Read file: docs/book/src/cronjob-tutorial/testdata/project/config/default/kustomization.yaml

이제 에러의 원인을 분석할 수 있습니다. 제공된 정보를 바탕으로 에러 원인을 설명드리겠습니다.

## 에러 원인 분석

### 1. 에러 메시지 분석
```
Warning   FailedMount         pod/project-controller-manager-5c447ddd54-557rs    MountVolume.SetUp failed for volume "webhook-certs" : secret "webhook-server-cert" not found
```

이 에러는 다음을 의미합니다:
- **pod**: `project-controller-manager`가 시작하려고 하지만
- **volume**: `webhook-certs` 볼륨을 마운트하는데 실패
- **원인**: `webhook-server-cert`라는 Secret이 존재하지 않음

### 2. 구성 파일 분석

**manager_webhook_patch.yaml**에서 webhook 인증서 설정을 확인했습니다:
```yaml
# webhook-certs 볼륨이 webhook-server-cert Secret을 참조
value:
  name: webhook-certs
  secret:
    secretName: webhook-server-cert  # <- 이 Secret이 없음
```

**certificate-webhook.yaml**에서 cert-manager 설정을 확인했습니다:
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: serving-cert
spec:
  secretName: webhook-server-cert  # <- 이 Secret을 생성해야 함
  issuerRef:
    kind: Issuer
    name: selfsigned-issuer
```

### 3. 근본 원인

**cert-manager가 설치되지 않았거나 제대로 작동하지 않음**

- `kustomization.yaml`에서 cert-manager 관련 설정이 활성화되어 있지만
- 실제 클러스터에 cert-manager가 설치되지 않았거나
- cert-manager가 Certificate 리소스를 처리하지 못해서 `webhook-server-cert` Secret이 생성되지 않음

### 4. 해결 방법

#### 방법 1: cert-manager 설치
```bash
# cert-manager 설치
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# 설치 확인
kubectl get pods -n cert-manager
```

#### 방법 2: 수동으로 Secret 생성 (테스트용)
```bash
# 자체 서명 인증서 생성
openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes -out webhook.crt -keyout webhook.key -subj "/CN=webhook-service.project-system.svc"

# Secret 생성
kubectl create secret tls webhook-server-cert \
  --cert=webhook.crt \
  --key=webhook.key \
  -n project-system
```

#### 방법 3: webhook 비활성화 (임시 해결책)
`config/default/kustomization.yaml`에서 webhook 관련 설정을 주석 처리:
```yaml
# - ../webhook  # <- 주석 처리
# - ../certmanager  # <- 주석 처리
```

### 5. 권장사항

운영 환경에서는 **방법 1 (cert-manager 설치)**을 강력히 권장합니다. cert-manager는 자동으로 인증서를 갱신하고 관리해주기 때문입니다.


## 문제 해결 

`config/default/kustomization.yaml`에서 webhook, certmanager 관련 설정을 활성화
```bash
[kauri@kdev project]$ git diff config/
diff --git a/config/default/kustomization.yaml b/config/default/kustomization.yaml
index bd70d1e..7adecf3 100644
--- a/config/default/kustomization.yaml
+++ b/config/default/kustomization.yaml
@@ -22,7 +22,7 @@ resources:
 # crd/kustomization.yaml
 - ../webhook
 # [CERTMANAGER] To enable cert-manager, uncomment all sections with 'CERTMANAGER'. 'WEBHOOK' components are required.
-#- ../certmanager
+- ../certmanager
 # [PROMETHEUS] To enable prometheus monitor, uncomment all sections with 'PROMETHEUS'.
 #- ../prometheus
 # [METRICS] Expose the controller manager metrics service.
[kauri@kdev project]$ 
```