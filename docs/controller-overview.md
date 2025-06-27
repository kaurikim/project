![The Kubebuilder Book 로고](https://book.kubebuilder.io/logos/logo-single-line.png)

# 컨트롤러 개요

컨트롤러는 Kubernetes 및 모든 오퍼레이터의 핵심입니다.
컨트롤러의 역할은 특정 객체(Object)에 대해 실제 세계의 상태(클러스터 상태뿐만 아니라 Kubelet의 실행 중인 컨테이너나 클라우드 제공자의 로드밸런서 같은 외부 상태)가 객체에 정의된 원하는 상태(desired state)와 일치하도록 보장하는 것입니다. 각 컨트롤러는 하나의 루트 Kind에 집중하지만, 다른 Kind와도 상호작용할 수 있습니다.

이 과정을 \*\*리콘실(reconciling)\*\*이라고 부릅니다.

`controller-runtime`에서는 특정 Kind에 대한 리콘실 로직을 [Reconciler](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile)라고 합니다. Reconciler는 객체의 이름을 입력으로 받아(예: 오류가 발생했거나 `HorizontalPodAutoscaler`처럼 주기적으로 동작하는 컨트롤러의 경우) 다시 시도해야 하는지 여부를 반환합니다.

```go
// Apache License
//
// Copyright 2022.
// Licensed under the Apache License, Version 2.0 (the “License”);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an “AS IS” BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
// First, we start out with some standard imports.
// As before, we need the core controller-runtime library,
// as well as the client package, and the package for our API types.

package controllers

import (
    "context"
    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    logf "sigs.k8s.io/controller-runtime/pkg/log"

    batchv1 "tutorial.kubebuilder.io/project/api/v1"
)
```

다음으로, kubebuilder는 기본 `Reconciler` 구조체를 스캐폴딩했습니다. 대부분의 Reconciler는 로깅과 객체 조회 기능이 필요하므로, 이 필드들이 기본으로 포함되어 있습니다.

```go
// CronJobReconciler는 CronJob 객체를 리콘실합니다.
type CronJobReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}
```

대부분의 컨트롤러는 결국 클러스터 내에서 실행되므로, 권한(Role)을 명시해야 합니다. `controller-tools`의 [RBAC 마커](https://book.kubebuilder.io/reference/markers/rbac.md)를 통해 필요한 최소 권한을 선언하며, 기능이 추가되면 이 부분을 다시 검토해야 합니다.

```go
// +kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/status,verbs=get;update;patch
```

위 마커를 기반으로 `config/rbac/role.yaml`의 ClusterRole 매니페스트가 `controller-gen`을 통해 생성됩니다.

```bash
make manifests
```

> **참고:** 오류가 발생하면, 오류 메시지에 제시된 명령을 실행한 후 다시 `make manifests`를 실행하세요.

`Reconcile` 메서드는 실제로 단일 이름의 객체에 대한 리콘실 작업을 수행합니다. 우리의 [Request](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile?tab=doc#Request)는 객체 이름만 포함하지만, `client`를 사용해 캐시에서 해당 객체를 가져올 수 있습니다. 빈 `Result`와 `nil` 에러를 반환하면 controller-runtime에 이 객체를 성공적으로 리콘실했으며, 변경사항이 있을 때까지 재시도할 필요가 없음을 알립니다.

```go
func (r *CronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    _ = logf.FromContext(ctx)

    // 여기에 로직을 구현하세요

    return ctrl.Result{}, nil
}
```

대부분의 컨트롤러는 로그를 기록할 로깅 핸들러와 요청 취소, 추적을 지원하는 [context](https://golang.org/pkg/context/)가 필요합니다. `controller-runtime`은 [logr](https://github.com/go-logr/logr) 라이브러리를 통해 구조화된 로깅을 제공하며, 키-값 쌍을 정적 메시지에 연결하는 방식으로 작동합니다.

```go
// 예시: 로그 핸들러 생성
_ = logf.FromContext(ctx)
```

마지막으로, 이 `Reconciler`를 매니저에 등록하여 매니저가 시작될 때 함께 실행되도록 합니다. 현재는 이 `Reconciler`가 `CronJob` 리소스를 대상으로 동작함을 지정하며, 이후에는 관련 객체도 관찰 대상으로 추가할 수 있습니다.

```go
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&batchv1.CronJob{}).
        Complete(r)
}
```

기본 `Reconciler` 구조를 살펴보았으니, 이제 `CronJob`에 대한 구체적인 로직을 구현해보겠습니다!
