![The Kubebuilder Book 로고](https://book.kubebuilder.io/logos/logo-single-line.png)

# 간단한 여담: 나머지 파일들은 무엇인가요?

만약 [api/v1/](https://github.com/kubernetes-sigs/kubebuilder/tree/master/docs/book/src/cronjob-tutorial/testdata/project/api/v1) 디렉터리의 나머지 파일들을 살펴봤다면, `cronjob_types.go` 외에 `groupversion_info.go`와 `zz_generated.deepcopy.go`라는 두 개의 추가 파일이 있는 것을 발견했을 겁니다.

이 두 파일은 **절대로 수정할 필요가 없**습니다(`groupversion_info.go`는 그대로 유지되고, `zz_generated.deepcopy.go`는 자동 생성되기 때문). 하지만, 어떤 내용을 담고 있는지 알아두면 유용합니다.

---

## groupversion\_info.go

`groupversion_info.go`에는 해당 그룹-버전에 대한 **공통 메타데이터**가 담겨 있습니다.
자세한 원본 파일은 [project/api/v1/groupversion\_info.go](https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/project/api/v1/groupversion_info.go)에서 확인할 수 있습니다.

```go
// Package v1 contains API Schema definitions for the batch v1 API group.
// +kubebuilder:object:generate=true
// +groupName=batch.tutorial.kubebuilder.io
package v1

import (
    "k8s.io/apimachinery/pkg/runtime/schema"
    "sigs.k8s.io/controller-runtime/pkg/scheme"
)
```

1. **패키지 수준 마커**

   * `// +kubebuilder:object:generate=true`
     이 패키지에 Kubernetes 객체가 있음을 나타내며, `object` 생성기가 사용합니다.
   * `// +groupName=batch.tutorial.kubebuilder.io`
     이 패키지가 속한 API 그룹 이름을 지정하며, CRD 생성기가 올바른 메타데이터를 생성할 때 활용합니다.

2. **Scheme 설정용 변수**
   아래 코드는 이 그룹-버전의 타입들을 Scheme에 쉽게 등록하기 위한 관례(convention)입니다.

   ```go
   var (
       // GroupVersion은 이 객체들을 등록할 때 사용되는 그룹-버전입니다.
       GroupVersion = schema.GroupVersion{Group: "batch.tutorial.kubebuilder.io", Version: "v1"}

       // SchemeBuilder는 Go 타입을 GVK 스킴에 추가하는 데 사용됩니다.
       SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

       // AddToScheme은 이 그룹-버전의 타입들을 주어진 스킴에 등록하는 함수입니다.
       AddToScheme = SchemeBuilder.AddToScheme
   )
   ```

---

## zz\_generated.deepcopy.go

`zz_generated.deepcopy.go`에는 **`runtime.Object` 인터페이스** 구현 부분이 자동으로 생성되어 있습니다. 이 인터페이스는 모든 루트 타입(root types)이 Kubernetes의 Kind를 나타내도록 하는 데 핵심이 되는 `DeepCopyObject()` 메서드를 포함합니다.

또한, `object` 생성기는 각 루트 타입과 그 하위 타입 각각에 대해 다음과 같은 편리한 메서드도 생성합니다:

* `DeepCopy()`
* `DeepCopyInto()`

이 메서드들은 객체를 안전하게 복제(deep copy)할 때 사용됩니다.
