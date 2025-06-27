![The Kubebuilder Book 로고](https://book.kubebuilder.io/logos/logo-single-line.png)

# 새로운 API 추가

새로운 Kind와 해당 컨트롤러를 스캐폴딩(scaffold)하려면, `kubebuilder create api` 명령을 사용합니다(지난 장을 참고하셨다면 기억나시죠?):

```bash
kubebuilder create api --group batch --version v1 --kind CronJob
```

`Create Resource`와 `Create Controller`에 대해 각각 `y`를 입력하세요.

각 그룹-버전에 대해 이 명령을 처음 호출하면, 해당 그룹-버전용 디렉터리가 생성됩니다.
이 예시에서는 [api/v1/](https://github.com/kubernetes-sigs/kubebuilder/tree/master/docs/book/src/cronjob-tutorial/testdata/project/api/v1) 디렉터리가 생성되며, 이는 `batch.tutorial.kubebuilder.io/v1`에 해당합니다(초반 [도메인 설정](https://book.kubebuilder.io/cronjob-tutorial/cronjob-tutorial) 참고).

또한 `CronJob` Kind에 대응하는 `api/v1/cronjob_types.go` 파일이 추가되었습니다. 다른 Kind로 명령을 실행할 때마다 해당 Kind에 대응하는 새로운 파일이 생성됩니다.

이제 기본으로 생성된 내용을 살펴보고, 실제 구현을 진행해보겠습니다.

다음은 스캐폴딩된 [`emptyapi.go`](https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/emptyapi.go) 파일의 기본 내용입니다:

```go
// Apache License
//
// Copyright 2022 The Kubernetes authors.
//
// Licensed under the Apache License, Version 2.0 (the “License”); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License
// is distributed on an “AS IS” BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.

package v1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required. Any new fields you add must have json tags
// for the fields to be serialized.

// CronJobSpec defines the desired state of CronJob
type CronJobSpec struct {
    // INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
    // Important: Run "make" to regenerate code after modifying this file
}

// CronJobStatus defines the observed state of CronJob
type CronJobStatus struct {
    // INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
    // Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CronJob is the Schema for the cronjobs API
type CronJob struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   CronJobSpec   `json:"spec,omitempty"`
    Status CronJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CronJobList contains a list of CronJob
type CronJobList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []CronJob `json:"items"`
}

func init() {
    SchemeBuilder.Register(&CronJob{}, &CronJobList{})
}
```

이제 기본 구조를 이해했으니, 다음 단계에서 `Spec`과 `Status`를 채우며 API 디자인을 진행해보겠습니다!
