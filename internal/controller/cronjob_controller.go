/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// CronJobReconciler reconciles a CronJob object
type CronJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Clock
}

// Clock은 현재 시간을 반환하는 인터페이스입니다.
// 테스트에서 시간 흐름을 조작할 때 사용합니다.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (_ realClock) Now() time.Time { return time.Now() } //nolint:staticcheck

// +kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

/*
Now, we get to the heart of the controller -- the reconciler logic.
*/
var (
	scheduledTimeAnnotation = "batch.tutorial.kubebuilder.io/scheduled-at"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CronJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *CronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	/*
	   ### 1: 이름으로 CronJob 불러오기

	   클라이언트를 사용하여 CronJob을 가져옵니다. 모든 클라이언트 메서드는
	   첫 번째 인자로 context(취소를 허용하기 위함)를, 마지막 인자로는
	   대상 객체를 받습니다. Get 메서드는 약간 특별해서 중간 인자로
	   [`NamespacedName`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client?tab=doc#ObjectKey)
	   를 받습니다(대부분의 메서드는 중간 인자가 없으며, 아래에서 볼 수 있습니다).

	   많은 클라이언트 메서드는 마지막에 가변 옵션을 받기도 합니다.
	*/
	var cronJob batchv1.CronJob
	if err := r.Get(ctx, req.NamespacedName, &cronJob); err != nil {
		log.Error(err, "unable to fetch CronJob")
		// 즉시 재큐로 수정할 수 없으므로 not-found 오류는 무시합니다
		// (새 알림을 기다려야 함). 삭제된 요청에서도 받을 수 있습니다.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	/*
		### 2: 모든 활성 Jobs 나열 및 상태 업데이트

		상태를 완전히 업데이트하려면 이 CronJob에 속한 모든 자식 Job을
		해당 네임스페이스에서 나열해야 합니다.
		Get과 마찬가지로 List 메서드를 사용하여 자식 Job을 나열할 수 있습니다.
		네임스페이스와 필드 매칭을 설정하기 위해 가변 옵션을 사용하는데,
		이는 아래에서 설정한 인덱스 조회이기도 합니다.
	*/
	var childJobs kbatch.JobList
	if err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Jobs")
		return ctrl.Result{}, err
	}

	/*
	   <aside class="note">

	   <h1>이 인덱스는 무엇에 관한 것인가요?</h1>

	   <p>리콘실러는 상태를 위해 CronJob이 소유한 모든 Job을 가져옵니다. CronJob 수가 증가함에 따라,
	   이들을 모두 필터링해야 하므로 조회 속도가 상당히 느려질 수 있습니다. 더 효율적인 조회를 위해,
	   이 Job들은 컨트롤러 이름별로 로컬에 인덱싱됩니다. 캐시된 Job 객체에 jobOwnerKey 필드가 추가되며,
	   이 키는 소유 컨트롤러를 참조하여 인덱스로 작동합니다. 이 문서 후반부에서 매니저를
	   구성하여 실제로 이 필드를 인덱싱하도록 설정할 것입니다.</p>

	   </aside>

	   우리가 소유한 모든 Job을 가져오면, 이를 활성, 성공, 실패 Job으로 분리하고,
	   가장 최근 실행 시간을 추적하여 상태에 기록합니다. 상태는 시스템의 실제 상태로부터
	   재구성할 수 있어야 하므로, 루트 객체의 상태를 직접 읽는 것은 일반적으로 권장되지 않습니다.
	   대신 매번 실행 시 상태를 재구성해야 합니다. 여기서도 동일한 방식으로 처리할 것입니다.

	   status 조건을 사용하여 Job이 "완료"되었는지, 성공했는지 실패했는지 확인할 수 있습니다.
	   이후 코드를 깔끔하게 유지하기 위해 해당 로직을 헬퍼 함수로 분리하겠습니다.
	*/

	// 활성 Job 목록 찾기
	var activeJobs []*kbatch.Job
	var successfulJobs []*kbatch.Job
	var failedJobs []*kbatch.Job
	var mostRecentTime *time.Time // 마지막 실행 시간을 찾아 상태를 업데이트합니다

	/*
		"Complete" 또는 "Failed" 조건이 true로 표시된 경우 작업이 "완료"된 것으로 간주합니다.
		상태 조건을 사용하면 다른 사람과 컨트롤러가 완료 및 상태를 확인할 수 있는
		확장 가능한 상태 정보를 객체에 추가할 수 있습니다.
	*/
	isJobFinished := func(job *kbatch.Job) (bool, kbatch.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == kbatch.JobComplete || c.Type == kbatch.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}

		return false, ""
	}
	// +kubebuilder:docs-gen:collapse=isJobFinished

	/*
		작업 생성 중에 추가한 주석에서 예정된 시간을 추출하는 헬퍼를 사용할 것입니다.
	*/
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
	// +kubebuilder:docs-gen:collapse=getScheduledTimeForJob

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

		// 시작 시간을 주석에 저장하므로 활성 작업 자체에서 재구성할 것입니다.
		scheduledTimeForJob, err := getScheduledTimeForJob(&job)
		if err != nil {
			log.Error(err, "unable to parse schedule time for child job", "job", &job)
			continue
		}
		if scheduledTimeForJob != nil {
			if mostRecentTime == nil || mostRecentTime.Before(*scheduledTimeForJob) {
				mostRecentTime = scheduledTimeForJob
			}
		}
	}

	if mostRecentTime != nil {
		cronJob.Status.LastScheduleTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		cronJob.Status.LastScheduleTime = nil
	}
	cronJob.Status.Active = nil
	for _, activeJob := range activeJobs {
		jobRef, err := ref.GetReference(r.Scheme, activeJob)
		if err != nil {
			log.Error(err, "unable to make reference to active job", "job", activeJob)
			continue
		}
		cronJob.Status.Active = append(cronJob.Status.Active, *jobRef)
	}

	/*
		여기서는 디버깅을 위해 약간 더 높은 로깅 레벨에서 관찰한 작업 수를 로그로 남깁니다.
		포맷 문자열을 사용하는 대신 고정된 메시지를 사용하고 추가 정보를 키-값 쌍으로
		첨부하는 방법을 주목하세요. 이렇게 하면 로그 라인을 필터링하고 쿼리하기가 더 쉬워집니다.
	*/
	log.V(1).Info("job count", "active jobs", len(activeJobs), "successful jobs", len(successfulJobs), "failed jobs", len(failedJobs))

	/*
	   수집한 데이터를 사용하여 CRD의 상태를 업데이트합니다.
	   이전과 마찬가지로 클라이언트를 사용하며,
	   상태 서브리소스를 업데이트하려면 클라이언트의 `Status` 부분에서
	   `Update` 메서드를 호출합니다.

	   상태 서브리소스는 spec 변경을 무시하므로
	   다른 업데이트와 충돌할 가능성이 적고, 별도의 권한으로 관리할 수 있습니다.
	*/
	if err := r.Status().Update(ctx, &cronJob); err != nil {
		log.Error(err, "unable to update CronJob status")
		return ctrl.Result{}, err
	}

	/*
	   상태를 업데이트한 후, 스펙에서 원하는 상태와 실제 세계의 상태가 일치하는지 확인할 수 있습니다.

	   ### 3: 보관 기록 제한에 따라 오래된 Job 정리

	   먼저 오래된 Job을 정리하여 너무 많이 남지 않도록 합니다.
	*/

	// 참고: 이들의 삭제는 '최선의 노력'입니다.
	// 특정 삭제에 실패하더라도 삭제를 완료하기 위해 재큐하지 않습니다.
	if cronJob.Spec.FailedJobsHistoryLimit != nil {
		sort.Slice(failedJobs, func(i, j int) bool {
			if failedJobs[i].Status.StartTime == nil {
				return failedJobs[j].Status.StartTime != nil
			}
			return failedJobs[i].Status.StartTime.Before(failedJobs[j].Status.StartTime)
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

	if cronJob.Spec.SuccessfulJobsHistoryLimit != nil {
		sort.Slice(successfulJobs, func(i, j int) bool {
			if successfulJobs[i].Status.StartTime == nil {
				return successfulJobs[j].Status.StartTime != nil
			}
			return successfulJobs[i].Status.StartTime.Before(successfulJobs[j].Status.StartTime)
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

	/*
	   ### 4: 일시 중단 여부 확인

	   이 객체가 일시 중단(Suspend) 상태라면 어떤 Job도 실행하지 않으므로
	   여기서 종료합니다. 이는 실행 중인 Job에 문제가 발생하여
	   클러스터를 조사하거나 수정하기 위해 실행을 일시 중지하되
	   객체 자체는 삭제하지 않으려는 경우에 유용합니다.
	*/

	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		log.V(1).Info("cronjob suspended, skipping")
		return ctrl.Result{}, nil
	}

	/*
	   ### 5: 다음 예정 실행 시간 계산

	   일시 중단 상태가 아니라면 다음 예정 실행 시간을 계산하고,
	   아직 처리하지 않은 실행이 있는지 확인해야 합니다.
	*/

	/*
	   유용한 cron 라이브러리를 사용하여 다음 예정 시간을 계산합니다.
	   마지막 실행 시간부터, 또는 마지막 실행 시간을 찾을 수 없으면
	   CronJob이 생성된 시점부터 계산을 시작합니다.

	   누락된 실행이 너무 많고 데드라인이 설정되어 있지 않으면,
	   컨트롤러 재시작 또는 중단 시 문제가 발생하지 않도록
	   여기서 중단합니다.

	   그렇지 않으면 누락된 실행(가장 최근 실행)과 다음 실행 시간을 반환하여,
	   언제 다시 리콘실해야 할지 알 수 있게 합니다.
	*/
	getNextSchedule := func(cronJob *batchv1.CronJob, now time.Time) (lastMissed time.Time, next time.Time, err error) {
		sched, err := cron.ParseStandard(cronJob.Spec.Schedule)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("unparseable schedule %q: %w", cronJob.Spec.Schedule, err)
		}

		// for optimization purposes, cheat a bit and start from our last observed run time
		// we could reconstitute this here, but there's not much point, since we've
		// just updated it.
		var earliestTime time.Time
		if cronJob.Status.LastScheduleTime != nil {
			earliestTime = cronJob.Status.LastScheduleTime.Time
		} else {
			earliestTime = cronJob.CreationTimestamp.Time
		}
		if cronJob.Spec.StartingDeadlineSeconds != nil {
			// controller is not going to schedule anything below this point
			schedulingDeadline := now.Add(-time.Second * time.Duration(*cronJob.Spec.StartingDeadlineSeconds))

			if schedulingDeadline.After(earliestTime) {
				earliestTime = schedulingDeadline
			}
		}
		if earliestTime.After(now) {
			return time.Time{}, sched.Next(now), nil
		}

		starts := 0
		for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
			lastMissed = t
			// An object might miss several starts. For example, if
			// controller gets wedged on Friday at 5:01pm when everyone has
			// gone home, and someone comes in on Tuesday AM and discovers
			// the problem and restarts the controller, then all the hourly
			// jobs, more than 80 of them for one hourly scheduledJob, should
			// all start running with no further intervention (if the scheduledJob
			// allows concurrency and late starts).
			//
			// However, if there is a bug somewhere, or incorrect clock
			// on controller's server or apiservers (for setting creationTimestamp)
			// then there could be so many missed start times (it could be off
			// by decades or more), that it would eat up all the CPU and memory
			// of this controller. In that case, we want to not try to list
			// all the missed start times.
			starts++
			if starts > 100 {
				// We can't get the most recent times so just return an empty slice
				return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start times (> 100). Set or decrease .spec.startingDeadlineSeconds or check clock skew.") //nolint:staticcheck
			}
		}
		return lastMissed, sched.Next(now), nil
	}
	// +kubebuilder:docs-gen:collapse=getNextSchedule

	// figure out the next times that we need to create
	// jobs at (or anything we missed).
	missedRun, nextRun, err := getNextSchedule(&cronJob, r.Now())
	if err != nil {
		log.Error(err, "unable to figure out CronJob schedule")
		// we don't really care about requeuing until we get an update that
		// fixes the schedule, so don't return an error
		return ctrl.Result{}, nil
	}

	/*
		We'll prep our eventual request to requeue until the next job, and then figure
		out if we actually need to run.
	*/
	scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(r.Now())} // save this so we can re-use it elsewhere
	log = log.WithValues("now", r.Now(), "next run", nextRun)

	/*
		### 6: Run a new job if it's on schedule, not past the deadline, and not blocked by our concurrency policy

		If we've missed a run, and we're still within the deadline to start it, we'll need to run a job.
	*/
	if missedRun.IsZero() {
		log.V(1).Info("no upcoming scheduled times, sleeping until next")
		return scheduledResult, nil
	}

	// make sure we're not too late to start the run
	log = log.WithValues("current run", missedRun)
	tooLate := false
	if cronJob.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*cronJob.Spec.StartingDeadlineSeconds) * time.Second).Before(r.Now())
	}
	if tooLate {
		log.V(1).Info("missed starting deadline for last run, sleeping till next")
		// TODO(directxman12): events
		return scheduledResult, nil
	}

	/*
		If we actually have to run a job, we'll need to either wait till existing ones finish,
		replace the existing ones, or just add new ones.  If our information is out of date due
		to cache delay, we'll get a requeue when we get up-to-date information.
	*/
	// figure out how to run this job -- concurrency policy might forbid us from running
	// multiple at the same time...
	if cronJob.Spec.ConcurrencyPolicy == batchv1.ForbidConcurrent && len(activeJobs) > 0 {
		log.V(1).Info("concurrency policy blocks concurrent runs, skipping", "num active", len(activeJobs))
		return scheduledResult, nil
	}

	// ...or instruct us to replace existing ones...
	if cronJob.Spec.ConcurrencyPolicy == batchv1.ReplaceConcurrent {
		for _, activeJob := range activeJobs {
			// we don't care if the job was already deleted
			if err := r.Delete(ctx, activeJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete active job", "job", activeJob)
				return ctrl.Result{}, err
			}
		}
	}

	/*
		Once we've figured out what to do with existing jobs, we'll actually create our desired job
	*/

	/*
		We need to construct a job based on our CronJob's template.  We'll copy over the spec
		from the template and copy some basic object meta.

		Then, we'll set the "scheduled time" annotation so that we can reconstitute our
		`LastScheduleTime` field each reconcile.

		Finally, we'll need to set an owner reference.  This allows the Kubernetes garbage collector
		to clean up jobs when we delete the CronJob, and allows controller-runtime to figure out
		which cronjob needs to be reconciled when a given job changes (is added, deleted, completes, etc).
	*/
	constructJobForCronJob := func(cronJob *batchv1.CronJob, scheduledTime time.Time) (*kbatch.Job, error) {
		// We want job names for a given nominal start time to have a deterministic name to avoid the same job being created twice
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
	// +kubebuilder:docs-gen:collapse=constructJobForCronJob

	// actually make the job...
	job, err := constructJobForCronJob(&cronJob, missedRun)
	if err != nil {
		log.Error(err, "unable to construct job from template")
		// don't bother requeuing until we get a change to the spec
		return scheduledResult, nil
	}

	// ...and create it on the cluster
	if err := r.Create(ctx, job); err != nil {
		log.Error(err, "unable to create Job for CronJob", "job", job)
		return ctrl.Result{}, err
	}

	log.V(1).Info("created Job for CronJob run", "job", job)

	/*
		### 7: Requeue when we either see a running job or it's time for the next scheduled run

		Finally, we'll return the result that we prepped above, that says we want to requeue
		when our next run would need to occur.  This is taken as a maximum deadline -- if something
		else changes in between, like our job starts or finishes, we get modified, etc, we might
		reconcile again sooner.
	*/
	// we'll requeue once we see the running job, and update our status
	return scheduledResult, nil
}

/*
### Setup

Finally, we'll update our setup.  In order to allow our reconciler to quickly
look up Jobs by their owner, we'll need an index.  We declare an index key that
we can later use with the client as a pseudo-field name, and then describe how to
extract the indexed value from the Job object.  The indexer will automatically take
care of namespaces for us, so we just have to extract the owner name if the Job has
a CronJob owner.

Additionally, we'll inform the manager that this controller owns some Jobs, so that it
will automatically call Reconcile on the underlying CronJob when a Job changes, is
deleted, etc.
*/
var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = batchv1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// set up a real clock, since we're not in a test
	if r.Clock == nil {
		r.Clock = realClock{}
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kbatch.Job{}, jobOwnerKey, func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*kbatch.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "CronJob" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Owns(&kbatch.Job{}).
		Named("cronjob").
		Complete(r)
}
