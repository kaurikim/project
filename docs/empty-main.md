
![The Kubebuilder Book 로고](https://book.kubebuilder.io/logos/logo-single-line.png)

# 모든 여정에는 시작이 필요하고, 모든 프로그램에는 main 함수가 필요합니다

```go
// Apache License
//
// Copyright 2022 The Kubernetes authors.
//
// Licensed under the Apache License, Version 2.0 (the “License”); you may not use this file except in compliance with the License.  
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an “AS IS” BASIS,  
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing  
// permissions and limitations under the License.
```

이 패키지는 몇 가지 기본 import로 시작합니다. 특히:

* 핵심 [controller-runtime 라이브러리](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
* 기본 controller-runtime 로깅인 [Zap](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/log/zap) (나중에 자세히 다룹니다)

```go
package main

import (
    "flag"
    "os"

    // Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
    // to ensure that exec-entrypoint and run can make use of them.
    _ "k8s.io/client-go/plugin/pkg/client/auth"
    "k8s.io/apimachinery/pkg/runtime"
    utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    clientgoscheme "k8s.io/client-go/kubernetes/scheme"
    _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/cache"
    "sigs.k8s.io/controller-runtime/pkg/healthz"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"
    "sigs.k8s.io/controller-runtime/pkg/metrics/server"
    "sigs.k8s.io/controller-runtime/pkg/webhook"
    // +kubebuilder:scaffold:imports
)
```

모든 컨트롤러 집합에는 [Scheme](https://pkg.go.dev/k8s.io/apimachinery/pkg/runtime#Scheme)가 필요합니다. Scheme은 Kubernetes의 Group-Version-Kind(GVK)와 Go 타입 간 매핑을 제공합니다. API 정의를 작성할 때 이 부분을 더 자세히 다룰 예정입니다.

```go
var (
    scheme   = runtime.NewScheme()
    setupLog = ctrl.Log.WithName("setup")
)

func init() {
    utilruntime.Must(clientgoscheme.AddToScheme(scheme))
    // +kubebuilder:scaffold:scheme
}
```

이제 `main` 함수는 다음과 같은 역할을 수행합니다:

1. 메트릭 바인딩-주소, 헬스 프로브 바인딩-주소, 리더선출 여부와 같은 기본 플래그를 설정합니다.
2. [Manager](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager)를 인스턴스화합니다. Manager는 컨트롤러 실행, 공유 캐시 설정, API 서버 클라이언트 생성 등을 관리합니다(위 예제에서 `Scheme`을 알려주는 부분에 주목하세요).
3. Manager를 실행합니다. Manager는 graceful shutdown 신호를 받을 때까지 실행되므로, Kubernetes에서 Pod 종료 시 우아하게 정리됩니다.

아직 실제로 동작할 컨트롤러는 없지만, 코드 중간에 있는 `// +kubebuilder:scaffold:builder` 주석 위치를 기억해 두세요—곧 여기에 컨트롤러가 자동으로 추가됩니다.

```go
func main() {
    var metricsAddr string
    var enableLeaderElection bool
    var probeAddr string
    flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
    flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
    flag.BoolVar(&enableLeaderElection, "leader-elect", false,
        "Enable leader election for controller manager. "+
            "Enabling this will ensure there is only one active controller manager.")
    opts := zap.Options{
        Development: true,
    }
    opts.BindFlags(flag.CommandLine)
    flag.Parse()

    ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        Scheme: scheme,
        Metrics: server.Options{
            BindAddress: metricsAddr,
        },
        WebhookServer:          webhook.NewServer(webhook.Options{Port: 9443}),
        HealthProbeBindAddress: probeAddr,
        LeaderElection:         enableLeaderElection,
        LeaderElectionID:       "80807133.tutorial.kubebuilder.io",
    })
    if err != nil {
        setupLog.Error(err, "unable to start manager")
        os.Exit(1)
    }

    // +kubebuilder:scaffold:builder

    if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
        setupLog.Error(err, "unable to set up health check")
        os.Exit(1)
    }
    if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
        setupLog.Error(err, "unable to set up ready check")
        os.Exit(1)
    }

    setupLog.Info("starting manager")
    if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
        setupLog.Error(err, "problem running manager")
        os.Exit(1)
    }
}
```

위와 같이 `main.go`가 준비되었으니, 이제 API 스캐폴딩 단계로 넘어가겠습니다!
