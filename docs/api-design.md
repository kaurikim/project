![The Kubebuilder Book 로고](https://book.kubebuilder.io/logos/logo-single-line.png)

# API 설계

Kubernetes에서는 API를 설계할 때 몇 가지 규칙이 있습니다. 모든 직렬화된 필드는 `camelCase`여야 하므로, JSON struct 태그를 사용해 이를 지정합니다. 또한 `omitempty` struct 태그를 사용하여 필드가 비어 있을 때 직렬화에서 생략되도록 표시할 수 있습니다.

필드는 대부분의 기본형(primitive types)을 사용할 수 있지만, 숫자의 경우 API 호환성 유지를 위해 세 가지 형태만 허용됩니다. 정수는 `int32` 또는 `int64`를, 소수점(십진수) 표기는 `resource.Quantity`를 사용합니다.

---

## Quantity란?

`Quantity`는 고정된 표현을 갖는 십진수 표기법으로, 기계 간 이식성을 높이기 위해 사용됩니다. Kubernetes에서 파드의 리소스 요청(requests) 및 제한(limits)을 지정할 때 자주 보입니다. 개념적으로는 부동 소수점(floating point)과 유사하며, **유효숫자(significant)**, **기수(base)**, \*\*지수(exponent)\*\*로 구성됩니다.
사람이 읽기 편하도록, 정수와 접미사(suffix)를 조합해 값을 표현합니다.

* 예시: `2m`은 십진수로 `0.002`를 의미합니다.
* `2Ki`는 십진수로 `2048`, `2K`는 십진수로 `2000`을 의미합니다.
* 소수를 표현할 때는 정수와 `m` 접미사를 조합합니다: `2.5`는 `2500m`이 됩니다.

지원하는 기수는 두 가지입니다.

* **십진(decimal)**: 일반 SI 접미사(`M`, `K` 등) 사용
* **이진(binary)**: 메비(mebi) 접미사(`Mi`, `Ki` 등) 사용
  자세한 비교는 [megabytes vs mebibytes](https://en.wikipedia.org/wiki/Megabyte#Mebibyte_and_megabyte) 참고하세요.

또 다른 특별한 타입으로 `metav1.Time`이 있습니다. 이는 Go의 `time.Time`과 동일하게 작동하지만, 고정된 직렬화 형식을 유지합니다.

---

이제 CronJob 객체의 정의를 살펴보겠습니다!

[`project/api/v1/cronjob_types.go`](https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/project/api/v1/cronjob_types.go):

```go
// Apache License
//
// Copyright 2025 The Kubernetes authors.
//
// Licensed under the Apache License, Version 2.0 (the “License”); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an “AS IS” BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package v1

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be serialized.
```

### Spec 정의

`spec`는 원하는 상태(desired state)를 나타내며, 컨트롤러에 제공되는 모든 “입력”이 이곳에 담깁니다.

기본적으로 CronJob에는 다음 항목이 필요합니다:

* **스케줄** (Cron 형식)
* **Job 템플릿** (실제 실행될 Job 정의)

사용자 편의를 위한 추가 옵션:

* 작업 시작 기한(deadline) — 기한을 놓치면 다음 스케줄까지 대기
* 동시 실행 처리 방식 — 대기할지, 기존 작업을 중지할지, 둘 다 실행할지
* 실행 일시 중단 플래그 — 문제가 있을 때 사용
* 이전 작업 기록(history) 보관 개수 제한

참고: 컨트롤러는 자체 상태를 읽지 않으므로, 작업이 실행되었는지 확인하기 위해 **적어도 하나의 이전 Job** 정보를 활용합니다.

필드에 대한 기술적 메타데이터는 `// +kubebuilder:validation:...` 식의 마커를 사용하며, 이는 [controller-tools](https://github.com/kubernetes-sigs/controller-tools)에서 CRD 매니페스트를 생성할 때 활용됩니다.

```go
// CronJobSpec defines the desired state of CronJob.
type CronJobSpec struct {
	// +kubebuilder:validation:MinLength=0

	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	Schedule string `json:"schedule"`

	// +kubebuilder:validation:Minimum=0
	// Optional deadline in seconds for starting the job if it misses scheduled
	// time for any reason. Missed jobs executions will be counted as failed ones.
	// +optional
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`

	// Specifies how to treat concurrent executions of a Job.
	// Valid values are:
	// - "Allow" (default): allows CronJobs to run concurrently;
	// - "Forbid": forbids concurrent runs, skipping next run if previous run hasn't finished yet;
	// - "Replace": cancels currently running job and replaces it with a new one
	// +optional
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// This flag tells the controller to suspend subsequent executions, it does
	// not apply to already started executions. Defaults to false.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`

	// Specifies the job that will be created when executing a CronJob.
	JobTemplate batchv1.JobTemplateSpec `json:"jobTemplate"`

	// +kubebuilder:validation:Minimum=0
	// The number of successful finished jobs to retain.
	// This is a pointer to distinguish between explicit zero and not specified.
	// +optional
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// The number of failed finished jobs to retain.
	// This is a pointer to distinguish between explicit zero and not specified.
	// +optional
	FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty"`
}
```

### ConcurrencyPolicy 타입

`ConcurrencyPolicy`는 내부적으로 문자열이지만, 별도 타입으로 정의하여 문서화하고 재사용성을 높입니다.

```go
// ConcurrencyPolicy describes how the job will be handled.
// Only one of the following concurrent policies may be specified.
// If none of the following policies is specified, the default one
// is AllowConcurrent.
// +kubebuilder:validation:Enum=Allow;Forbid;Replace
type ConcurrencyPolicy string

const (
	// AllowConcurrent allows CronJobs to run concurrently.
	AllowConcurrent ConcurrencyPolicy = "Allow"
	// ForbidConcurrent forbids concurrent runs, skipping next run if previous
	// hasn't finished yet.
	ForbidConcurrent ConcurrencyPolicy = "Forbid"
	// ReplaceConcurrent cancels currently running job and replaces it with a new one.
	ReplaceConcurrent ConcurrencyPolicy = "Replace"
)
```

---

### Status 정의

`status`는 관찰된 상태(observed state)를 담습니다. 사용자나 다른 컨트롤러가 쉽게 조회할 정보를 정의합니다.

```go
// CronJobStatus defines the observed state of CronJob.
type CronJobStatus struct {
	// A list of pointers to currently running jobs.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// Information when was the last time the job was successfully scheduled.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
}
```

---

마지막으로, 남은 보일러플레이트를 살펴봅니다. 상태 서브리소스를 활성화하기 위해 `+kubebuilder:subresource:status` 어노테이션을 추가한 것 외에는 변경할 필요가 없습니다.

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CronJob is the Schema for the cronjobs API.
type CronJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronJobSpec   `json:"spec,omitempty"`
	Status CronJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CronJobList contains a list of CronJob.
type CronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CronJob{}, &CronJobList{})
}
```

이제 API가 준비되었으니, 실제 기능을 구현하기 위해 컨트롤러를 작성해야 합니다.
