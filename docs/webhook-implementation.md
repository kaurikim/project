# 기본값 적용/검증 웹훅 구현

CRD에 [어드미션 웹훅](#/reference/admission-webhook)을 구현하려면, `CustomDefaulter` 및(또는) `CustomValidator` 인터페이스만 구현하면 됩니다.

Kubebuilder는 다음 작업을 대신 처리해 줍니다:

1. 웹훅 서버 생성
2. 매니저에 서버가 추가되었는지 확인
3. 웹훅 핸들러 생성
4. 서버에 각 핸들러 등록

먼저, 우리 CRD(CronJob)에 대한 웹훅을 스캐폴딩합니다. 테스트 프로젝트에서 디폴팅 및 검증 웹훅을 사용할 예정이므로, `--defaulting` 및 `--programmatic-validation` 플래그를 지정해 다음 명령을 실행합니다:

```bash
kubebuilder create webhook --group batch --version v1 --kind CronJob --defaulting --programmatic-validation
```

이 명령은 웹훅 함수 스캐폴딩과 함께, `main.go`에서 매니저에 웹훅을 등록하는 코드를 생성해 줍니다.

<details>
<summary>Apache License</summary>

```
Copyright 2025 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the “License”);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an “AS IS” BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and limitations under the License.
```

</details>

<details>
<summary>Go imports</summary>

```go
package v1

import (
    "context"
    "fmt"

    "github.com/robfig/cron"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/runtime/schema"
    validationutils "k8s.io/apimachinery/pkg/util/validation"
    "k8s.io/apimachinery/pkg/util/validation/field"

    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    logf "sigs.k8s.io/controller-runtime/pkg/log"
    "sigs.k8s.io/controller-runtime/pkg/webhook"
    "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

    batchv1 "tutorial.kubebuilder.io/project/api/v1"
)
```

</details>

다음으로, 웹훅용 로거를 설정합니다.

```go
var cronjoblog = logf.Log.WithName("cronjob-resource")
```

그다음 매니저에 웹훅을 등록합니다.

```go
// SetupCronJobWebhookWithManager는 매니저에 CronJob 웹훅을 등록합니다.
func SetupCronJobWebhookWithManager(mgr ctrl.Manager) error {
    return ctrl.NewWebhookManagedBy(mgr).For(&batchv1.CronJob{}).
        WithValidator(&CronJobCustomValidator{}).
        WithDefaulter(&CronJobCustomDefaulter{
            DefaultConcurrencyPolicy:          batchv1.AllowConcurrent,
            DefaultSuspend:                    false,
            DefaultSuccessfulJobsHistoryLimit: 3,
            DefaultFailedJobsHistoryLimit:     1,
        }).
        Complete()
}
```

kubebuilder 마커를 사용해 변이(mutating) 웹훅 매니페스트를 생성합니다.
각 마커의 의미는 [여기](#/reference/markers/webhook)에서 확인할 수 있습니다.

```go
// +kubebuilder:webhook:path=/mutate-batch-tutorial-kubebuilder-io-v1-cronjob,mutating=true,failurePolicy=fail,sideEffects=None,groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=create;update,versions=v1,name=mcronjob-v1.kb.io,admissionReviewVersions=v1

// CronJobCustomDefaulter는 CronJob 커스텀 리소스 생성·업데이트 시 기본값을 설정합니다.
// +kubebuilder:object:generate=false
type CronJobCustomDefaulter struct {
    // CronJob 필드의 기본값
    DefaultConcurrencyPolicy          batchv1.ConcurrencyPolicy
    DefaultSuspend                    bool
    DefaultSuccessfulJobsHistoryLimit int32
    DefaultFailedJobsHistoryLimit     int32
}

var _ webhook.CustomDefaulter = &CronJobCustomDefaulter{}
```

`webhook.CustomDefaulter` 인터페이스 구현으로 CRD에 기본값을 설정합니다. `Default` 메서드는 리시버를 변경해 기본값을 적용해야 합니다.

```go
// Default는 webhook.CustomDefaulter를 구현하여 CronJob 기본값 웹훅을 등록합니다.
func (d *CronJobCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
    cronjob, ok := obj.(*batchv1.CronJob)
    if !ok {
        return fmt.Errorf("CronJob 객체를 기대했으나 %T 타입을 받았습니다", obj)
    }
    cronjoblog.Info("CronJob 기본값 설정", "name", cronjob.GetName())

    // 기본값 적용
    d.applyDefaults(cronjob)
    return nil
}

// applyDefaults는 CronJob 필드에 기본값을 적용합니다.
func (d *CronJobCustomDefaulter) applyDefaults(cronJob *batchv1.CronJob) {
    if cronJob.Spec.ConcurrencyPolicy == "" {
        cronJob.Spec.ConcurrencyPolicy = d.DefaultConcurrencyPolicy
    }
    if cronJob.Spec.Suspend == nil {
        cronJob.Spec.Suspend = new(bool)
        *cronJob.Spec.Suspend = d.DefaultSuspend
    }
    if cronJob.Spec.SuccessfulJobsHistoryLimit == nil {
        cronJob.Spec.SuccessfulJobsHistoryLimit = new(int32)
        *cronJob.Spec.SuccessfulJobsHistoryLimit = d.DefaultSuccessfulJobsHistoryLimit
    }
    if cronJob.Spec.FailedJobsHistoryLimit == nil {
        cronJob.Spec.FailedJobsHistoryLimit = new(int32)
        *cronJob.Spec.FailedJobsHistoryLimit = d.DefaultFailedJobsHistoryLimit
    }
}
```

선언적 검증만으로는 처리하기 어려운 추가 검증을 수행할 수 있습니다.
예를 들어, 긴 정규표현식 없이도 올바른 cron 스케줄 형식을 검증할 수 있습니다.

`webhook.CustomValidator` 인터페이스 구현 시, 검증 웹훅이 자동 생성됩니다.
`ValidateCreate`, `ValidateUpdate`, `ValidateDelete` 메서드는 각각 생성·업데이트·삭제 시 검증을 수행합니다.
여기서는 `ValidateCreate`와 `ValidateUpdate`에 동일한 검증 로직을 사용하고, 삭제 검증은 생략합니다.

```go
// +kubebuilder:webhook:path=/validate-batch-tutorial-kubebuilder-io-v1-cronjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=create;update,versions=v1,name=vcronjob-v1.kb.io,admissionReviewVersions=v1

// CronJobCustomValidator는 CronJob 리소스 생성·업데이트·삭제 시 검증을 담당합니다.
// +kubebuilder:object:generate=false
type CronJobCustomValidator struct {
    // TODO(user): 검증에 필요한 필드를 추가하세요.
}

var _ webhook.CustomValidator = &CronJobCustomValidator{}

// ValidateCreate는 CronJob 생성 시 검증을 수행합니다.
func (v *CronJobCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
    cronjob, ok := obj.(*batchv1.CronJob)
    if !ok {
        return nil, fmt.Errorf("CronJob 객체를 기대했으나 %T 타입을 받았습니다", obj)
    }
    cronjoblog.Info("CronJob 생성 검증", "name", cronjob.GetName())
    return nil, validateCronJob(cronjob)
}

// ValidateUpdate는 CronJob 업데이트 시 검증을 수행합니다.
func (v *CronJobCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
    cronjob, ok := newObj.(*batchv1.CronJob)
    if !ok {
        return nil, fmt.Errorf("새로운 객체를 CronJob으로 기대했으나 %T 타입을 받았습니다", newObj)
    }
    cronjoblog.Info("CronJob 업데이트 검증", "name", cronjob.GetName())
    return nil, validateCronJob(cronjob)
}

// ValidateDelete는 CronJob 삭제 시 검증을 수행합니다.
func (v *CronJobCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
    cronjob, ok := obj.(*batchv1.CronJob)
    if !ok {
        return nil, fmt.Errorf("CronJob 객체를 기대했으나 %T 타입을 받았습니다", obj)
    }
    cronjoblog.Info("CronJob 삭제 검증", "name", cronjob.GetName())
    // TODO(user): 삭제 시 검증 로직을 구현하세요.
    return nil, nil
}
```

이제 CronJob의 `metadata.name`과 `spec.schedule`을 검증하는 함수들을 구현합니다.

```go
// validateCronJob은 CronJob 객체의 필드를 검증합니다.
func validateCronJob(cronjob *batchv1.CronJob) error {
    var allErrs field.ErrorList
    if err := validateCronJobName(cronjob); err != nil {
        allErrs = append(allErrs, err)
    }
    if err := validateCronJobSpec(cronjob); err != nil {
        allErrs = append(allErrs, err)
    }
    if len(allErrs) == 0 {
        return nil
    }
    return apierrors.NewInvalid(
        schema.GroupKind{Group: "batch.tutorial.kubebuilder.io", Kind: "CronJob"},
        cronjob.Name, allErrs)
}

// validateCronJobSpec은 CronJob 스펙을 검증합니다.
func validateCronJobSpec(cronjob *batchv1.CronJob) *field.Error {
    // API 헬퍼를 사용해 구조화된 검증 오류를 반환합니다.
    return validateScheduleFormat(
        cronjob.Spec.Schedule,
        field.NewPath("spec").Child("schedule"))
}

// validateScheduleFormat은 cron 스케줄 형식이 올바른지 검증합니다.
func validateScheduleFormat(schedule string, fldPath *field.Path) *field.Error {
    if _, err := cron.ParseStandard(schedule); err != nil {
        return field.Invalid(fldPath, schedule, err.Error())
    }
    return nil
}
```

<details>
<summary>객체 이름 검증</summary>

```go
func validateCronJobName(cronjob *batchv1.CronJob) *field.Error {
    if len(cronjob.Name) > validationutils.DNS1035LabelMaxLength-11 {
        // 모든 Kubernetes 객체 이름은 DNS 하위 도메인 규격(최대 63자)을 따라야 합니다.
        // CronJob 컨트롤러는 Job 생성 시 11자 접미사(-$TIMESTAMP)를 추가합니다.
        // 따라서 CronJob 이름은 최대 63-11=52자여야 합니다.
        return field.Invalid(
            field.NewPath("metadata").Child("name"),
            cronjob.Name,
            "최대 52자여야 합니다",
        )
    }
    return nil
}
```

</details>

