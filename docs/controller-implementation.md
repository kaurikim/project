# 컨트롤러 구현하기

CronJob 컨트롤러의 기본 로직은 다음과 같습니다:

1. 명명된 CronJob 로드
2. 모든 활성 Job 나열 및 상태 업데이트
3. 기록 제한(history limits)에 따라 오래된 Job 정리
4. 일시 중단(suspend) 상태인지 확인(중단 상태면 이후 작업 수행 안 함)
5. 다음 실행 스케줄 획득
6. 스케줄에 맞고 데드라인이 지나지 않았으며 동시성 정책에 의해 차단되지 않은 경우 새 Job 실행
7. 실행 중인 Job(자동 완료 시) 또는 다음 실행 시점에 재큐(requeue)

[project/internal/controller/cronjob\_controller.go](https://sigs.k8s.io/kubebuilder/docs/book/src/cronjob-tutorial/testdata/project/internal/controller/cronjob_controller.go)

<details>
<summary>Apache License</summary>

Copyright 2025 The Kubernetes authors.
Licensed under the Apache License, Version 2.0 (the “License”); you may not use this file except in compliance with the License.
You may obtain a copy of the License at

```text
http://www.apache.org/licenses/LICENSE-2.0
```

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an “AS IS” BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and limitations under the License.

</details>

다음으로 몇 가지 import가 필요합니다. 아래에서 사용할 때마다 각 import를 설명합니다.

```go
package controller

import (
    "context"
    "fmt"
    "sort"
    "time"

    "github.com/robfig/cron"
    kbatch "k8s.io/api/batch/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    ref "k8s.io/client-go/tools/reference"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    logf "sigs.k8s.io/controller-runtime/pkg/log"

    batchv1 "tutorial.kubebuilder.io/project/api/v1"
)
```

다음으로, 테스트에서 시간 흐름을 시뮬레이션할 수 있도록 `Clock` 인터페이스가 필요합니다.

```go
// CronJobReconciler는 CronJob 객체를 재조정합니다.
type CronJobReconciler struct {
    client.Client
    Scheme *runtime.Scheme
    Clock
}
```

<details>
<summary>Clock</summary>

테스트 중에 시간 흐름을 컨트롤하기 위해 Clock을 모킹(mock)합니다. 실제 실행에서는 `time.Now()`를 호출합니다.

```go
type realClock struct{}

func (_ realClock) Now() time.Time { return time.Now() } //nolint:staticcheck

// Clock은 현재 시간을 반환하는 인터페이스입니다.
// 테스트 시 시간을 조작할 때 사용합니다.
type Clock interface {
    Now() time.Time
}
```

</details>

Job을 생성·관리하므로 몇 가지 RBAC 권한도 추가해야 합니다:

```go
// +kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
```

이제 재조정 로직의 핵심 부분입니다.

```go
var (
    scheduledTimeAnnotation = "batch.tutorial.kubebuilder.io/scheduled-at"
)

// Reconcile는 쿠버네티스 재조정 루프의 일부로, 클러스터의 현재 상태를
// 사용자가 지정한 상태에 가깝게 유지합니다.
// TODO(user): CronJob 객체의 spec과 실제 클러스터 상태를 비교하여,
// 필요한 연산을 수행하도록 Reconcile 함수를 수정하세요.
//
// 자세한 내용은 아래를 참고하세요:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
// nolint:gocyclo
func (r *CronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := logf.FromContext(ctx)
```

## 1. 이름으로 CronJob 로드하기

클라이언트를 사용해 CronJob 객체를 가져옵니다. 모든 클라이언트 메서드는 첫 인자로 `context`를, 마지막 인자로 객체를 받습니다.
`Get`은 중간에 `NamespacedName`을 받습니다.

```go
var cronJob batchv1.CronJob
if err := r.Get(ctx, req.NamespacedName, &cronJob); err != nil {
    log.Error(err, "unable to fetch CronJob")
    // NotFound 오류는 즉시 해결되지 않으므로 무시합니다.
    return ctrl.Result{}, client.IgnoreNotFound(err)
}
```

## 2. 모든 활성 Job 나열 및 상태 업데이트

자식 Job을 조회해서 상태를 재구성합니다. `List` 메서드에 namespace 및 필드 매칭 옵션을 주어
해당 CronJob의 하위 Job만 필터링합니다.

```go
var childJobs kbatch.JobList
if err := r.List(ctx, &childJobs,
    client.InNamespace(req.Namespace),
    client.MatchingFields{jobOwnerKey: req.Name},
); err != nil {
    log.Error(err, "unable to list child Jobs")
    return ctrl.Result{}, err
}
```

> **이 인덱스는 무엇에 관한 것인가?**
> 재조정기는 이 CronJob이 소유한 Job을 빠르게 조회하기 위해,
> Job 객체에 `jobOwnerKey` 필드를 인덱스로 추가합니다.
> 이후 manager 설정에서 이 필드를 인덱싱하도록 구성합니다.

```go
// 활성, 성공, 실패 Job을 구분하고, 가장 최근 실행 시간을 추적합니다.
var activeJobs []*kbatch.Job
var successfulJobs []*kbatch.Job
var failedJobs []*kbatch.Job
var mostRecentTime *time.Time
```

<details>
<summary>isJobFinished</summary>

Job의 `Status.Conditions`에서 완료(Complete) 또는 실패(Failed) 조건이 `true`면
종료된 것으로 간주합니다.

```go
isJobFinished := func(job *kbatch.Job) (bool, kbatch.JobConditionType) {
    for _, c := range job.Status.Conditions {
        if (c.Type == kbatch.JobComplete || c.Type == kbatch.JobFailed) &&
           c.Status == corev1.ConditionTrue {
            return true, c.Type
        }
    }
    return false, ""
}
```

</details>

<details>
<summary>getScheduledTimeForJob</summary>

Job 생성 시 추가한 annotation에서 스케줄된 시간을 파싱합니다.

```go
getScheduledTimeForJob := func(job *kbatch.Job) (*time.Time, error) {
    timeRaw := job.Annotations[scheduledTimeAnnotation]
    if len(timeRaw) == 0 {
        return nil, nil
    }
    timeParsed, err := time.Parse(time.RFC3339, timeRaw)
    if err != nil {
        return nil, err
    }
    return &timeParsed, nil
}
```

</details>

```go
for i, job := range childJobs.Items {
    _, finishedType := isJobFinished(&job)
    switch finishedType {
    case "": // 진행 중
        activeJobs = append(activeJobs, &childJobs.Items[i])
    case kbatch.JobFailed:
        failedJobs = append(failedJobs, &childJobs.Items[i])
    case kbatch.JobComplete:
        successfulJobs = append(successfulJobs, &childJobs.Items[i])
    }

    if schedTime, err := getScheduledTimeForJob(&job); err != nil {
        log.Error(err, "unable to parse schedule time for child job", "job", &job)
    } else if schedTime != nil &&
              (mostRecentTime == nil || mostRecentTime.Before(*schedTime)) {
        mostRecentTime = schedTime
    }
}

if mostRecentTime != nil {
    cronJob.Status.LastScheduleTime = &metav1.Time{Time: *mostRecentTime}
} else {
    cronJob.Status.LastScheduleTime = nil
}

cronJob.Status.Active = nil
for _, aj := range activeJobs {
    ref, err := ref.GetReference(r.Scheme, aj)
    if err != nil {
        log.Error(err, "unable to make reference to active job", "job", aj)
        continue
    }
    cronJob.Status.Active = append(cronJob.Status.Active, *ref)
}
```

```go
// 디버깅용 로그: 활성/성공/실패 Job 수 출력
log.V(1).Info("job count",
    "active jobs", len(activeJobs),
    "successful jobs", len(successfulJobs),
    "failed jobs", len(failedJobs),
)
```

수집한 데이터를 사용해 상태를 업데이트합니다. status 하위 리소스를 사용하면
spec 변경과 충돌 가능성이 줄어듭니다.

```go
if err := r.Status().Update(ctx, &cronJob); err != nil {
    log.Error(err, "unable to update CronJob status")
    return ctrl.Result{}, err
}
```

## 3. 기록 제한에 따라 오래된 Job 정리

```go
// 실패한 Job 정리 (best-effort)
if cronJob.Spec.FailedJobsHistoryLimit != nil {
    sort.Slice(failedJobs, func(i, j int) bool {
        if failedJobs[i].Status.StartTime == nil {
            return failedJobs[j].Status.StartTime != nil
        }
        return failedJobs[i].Status.StartTime.Before(*failedJobs[j].Status.StartTime)
    })
    for i, job := range failedJobs {
        if int32(i) >= int32(len(failedJobs))-*cronJob.Spec.FailedJobsHistoryLimit {
            break
        }
        if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
            log.Error(err, "unable to delete old failed job", "job", job)
        } else {
            log.V(0).Info("deleted old failed job", "job", job)
        }
    }
}

// 성공한 Job 정리
if cronJob.Spec.SuccessfulJobsHistoryLimit != nil {
    sort.Slice(successfulJobs, func(i, j int) bool {
        if successfulJobs[i].Status.StartTime == nil {
            return successfulJobs[j].Status.StartTime != nil
        }
        return successfulJobs[i].Status.StartTime.Before(*successfulJobs[j].Status.StartTime)
    })
    for i, job := range successfulJobs {
        if int32(i) >= int32(len(successfulJobs))-*cronJob.Spec.SuccessfulJobsHistoryLimit {
            break
        }
        if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
            log.Error(err, "unable to delete old successful job", "job", job)
        } else {
            log.V(0).Info("deleted old successful job", "job", job)
        }
    }
}
```

## 4. 일시 중단(suspend) 상태 확인

```go
if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
    log.V(1).Info("cronjob suspended, skipping")
    return ctrl.Result{}, nil
}
```

## 5. 다음 실행 스케줄 계산

<details>
<summary>getNextSchedule</summary>

Cron 스케줄을 파싱해 마지막 실행 시점 이후에 놓친(run missed) 시간과
다음 실행 시간을 계산합니다. 지나치게 놓친 횟수가 많으면 오류를 반환합니다.

```go
getNextSchedule := func(cronJob *batchv1.CronJob, now time.Time) (lastMissed time.Time, next time.Time, err error) {
    sched, err := cron.ParseStandard(cronJob.Spec.Schedule)
    if err != nil {
        return time.Time{}, time.Time{}, fmt.Errorf("unparseable schedule %q: %w", cronJob.Spec.Schedule, err)
    }
    var earliestTime time.Time
    if cronJob.Status.LastScheduleTime != nil {
        earliestTime = cronJob.Status.LastScheduleTime.Time
    } else {
        earliestTime = cronJob.CreationTimestamp.Time
    }
    if cronJob.Spec.StartingDeadlineSeconds != nil {
        deadline := now.Add(-time.Duration(*cronJob.Spec.StartingDeadlineSeconds) * time.Second)
        if deadline.After(earliestTime) {
            earliestTime = deadline
        }
    }
    if earliestTime.After(now) {
        return time.Time{}, sched.Next(now), nil
    }
    starts := 0
    for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
        lastMissed = t
        starts++
        if starts > 100 {
            return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start times (>100)...") //nolint:staticcheck
        }
    }
    return lastMissed, sched.Next(now), nil
}
```

</details>

```go
missedRun, nextRun, err := getNextSchedule(&cronJob, r.Now())
if err != nil {
    log.Error(err, "unable to figure out CronJob schedule")
    return ctrl.Result{}, nil
}
scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(r.Now())}
log = log.WithValues("now", r.Now(), "next run", nextRun)
```

## 6. 새 Job 실행 여부 판단

```go
if missedRun.IsZero() {
    log.V(1).Info("no upcoming scheduled times, sleeping until next")
    return scheduledResult, nil
}

tooLate := false
if cronJob.Spec.StartingDeadlineSeconds != nil {
    tooLate = missedRun.Add(time.Duration(*cronJob.Spec.StartingDeadlineSeconds)*time.Second).Before(r.Now())
}
if tooLate {
    log.V(1).Info("missed starting deadline for last run, sleeping till next")
    return scheduledResult, nil
}

// 동시성 정책 검사
if cronJob.Spec.ConcurrencyPolicy == batchv1.ForbidConcurrent && len(activeJobs) > 0 {
    log.V(1).Info("concurrency policy blocks concurrent runs, skipping", "num active", len(activeJobs))
    return scheduledResult, nil
}

// ReplaceConcurrent 정책이면 기존 활성 Job 삭제
if cronJob.Spec.ConcurrencyPolicy == batchv1.ReplaceConcurrent {
    for _, aj := range activeJobs {
        if err := r.Delete(ctx, aj, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
            log.Error(err, "unable to delete active job", "job", aj)
            return ctrl.Result{}, err
        }
    }
}
```

<details>
<summary>constructJobForCronJob</summary>

CronJob의 JobTemplate을 기반으로 실제 Job 객체를 생성합니다.

* deterministic한 이름 생성
* annotation에 스케줄 시간 기록
* owner reference 설정

```go
constructJobForCronJob := func(cronJob *batchv1.CronJob, scheduledTime time.Time) (*kbatch.Job, error) {
    name := fmt.Sprintf("%s-%d", cronJob.Name, scheduledTime.Unix())
    job := &kbatch.Job{
        ObjectMeta: metav1.ObjectMeta{
            Labels:      make(map[string]string),
            Annotations: make(map[string]string),
            Name:        name,
            Namespace:   cronJob.Namespace,
        },
        Spec: *cronJob.Spec.JobTemplate.Spec.DeepCopy(),
    }
    for k, v := range cronJob.Spec.JobTemplate.Annotations {
        job.Annotations[k] = v
    }
    job.Annotations[scheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)
    for k, v := range cronJob.Spec.JobTemplate.Labels {
        job.Labels[k] = v
    }
    if err := ctrl.SetControllerReference(cronJob, job, r.Scheme); err != nil {
        return nil, err
    }
    return job, nil
}
```

</details>

```go
job, err := constructJobForCronJob(&cronJob, missedRun)
if err != nil {
    log.Error(err, "unable to construct job from template")
    return scheduledResult, nil
}
if err := r.Create(ctx, job); err != nil {
    log.Error(err, "unable to create Job for CronJob", "job", job)
    return ctrl.Result{}, err
}
log.V(1).Info("created Job for CronJob run", "job", job)
```

## 7. 재큐 및 종료

```go
// 다음 실행 시점까지 재큐(requeue)하도록 결과 반환
return scheduledResult, nil
```

## Setup

Manager에 컨트롤러 및 인덱스를 등록합니다.

```go
var (
    jobOwnerKey = ".metadata.controller"
    apiGVStr    = batchv1.GroupVersion.String()
)

// SetupWithManager 설정
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
    if r.Clock == nil {
        r.Clock = realClock{}
    }
    if err := mgr.GetFieldIndexer().IndexField(
        context.Background(), &kbatch.Job{}, jobOwnerKey,
        func(rawObj client.Object) []string {
            job := rawObj.(*kbatch.Job)
            owner := metav1.GetControllerOf(job)
            if owner == nil ||
               owner.APIVersion != apiGVStr ||
               owner.Kind != "CronJob" {
                return nil
            }
            return []string{owner.Name}
        },
    ); err != nil {
        return err
    }
    return ctrl.NewControllerManagedBy(mgr).
        For(&batchv1.CronJob{}).
        Owns(&kbatch.Job{}).
        Named("cronjob").
        Complete(r)
}
```

이제 컨트롤러가 준비되었습니다. 클러스터에서 테스트한 후, 이슈가 없으면 배포하세요!
