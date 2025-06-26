![The Kubebuilder Book 로고](https://book.kubebuilder.io/logos/logo-single-line.png)

# 그룹, 버전, 그리고 종류(Kind), 오 마이!

API를 시작하기 전에 용어에 대해 조금 이야기해보겠습니다.

Kubernetes에서 API에 대해 이야기할 때, 흔히 네 가지 용어를 사용합니다: 그룹(groups), 버전(versions), 종류(kinds), 그리고 리소스(resources).

## 그룹과 버전

Kubernetes에서 API 그룹(API Group)은 관련된 기능의 모음일 뿐입니다. 각 그룹은 하나 이상의 버전을 가지며, 이름이 시사하듯 시간이 지나면서 API의 동작 방식을 변경할 수 있게 해줍니다.

## 종류(kinds)와 리소스(resources)

각 API 그룹-버전(group-version)은 하나 이상의 API 타입(API types)을 포함하며, 이를 우리는 ‘종류(Kind)’라고 부릅니다. Kind는 버전마다 형태가 바뀔 수 있지만, 각 버전이 서로 다른 데이터를 손실 없이 저장할 수 있어야 합니다(필드나 어노테이션(annotation)에 데이터를 저장하는 방식 등). 따라서 이전 API 버전을 사용해도 최신 데이터가 손실되거나 손상되지 않습니다. 자세한 내용은 [Kubernetes API 가이드라인](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md)을 참고하세요.

가끔 리소스(resources)라는 용어도 들을 수 있습니다. 리소스는 API에서 Kind의 사용 사례에 불과합니다. 보통 Kind와 리소스는 1:1 매핑됩니다. 예를 들어, `pods` 리소스는 `Pod` Kind에 대응합니다. 그러나 동일한 Kind가 여러 리소스로 반환될 때도 있습니다. 예를 들어, `Scale` Kind는 `deployments/scale`이나 `replicasets/scale`과 같은 모든 스케일 서브리소스에서 반환됩니다. 이것이 Kubernetes의 HorizontalPodAutoscaler가 다양한 리소스와 상호작용할 수 있게 해줍니다. 하지만 CRD를 사용할 때는 각 Kind가 단일 리소스에만 대응됩니다.

리소스는 항상 소문자로 표기되며, 관례상 Kind를 소문자로 바꾼 형태입니다.

## 그렇다면 Go에서는 어떻게 대응될까요?

특정 그룹-버전 내의 Kind를 지칭할 때는 이를 GroupVersionKind, 줄여서 GVK라고 부릅니다. 리소스도 마찬가지로 GroupVersionResource, 줄여서 GVR이라고 합니다. 곧 보겠지만, 각 GVK는 패키지 내의 특정 Go 루트 타입(root Go type)에 대응합니다.

이제 용어가 정리되었으니, 실제로 API를 생성해보겠습니다!

## 아, 그런데 Scheme은 뭔가요?

이전에 봤던 `Scheme`은 주어진 GVK가 어떤 Go 타입에 대응되는지를 추적하는 방식일 뿐입니다(문서를 모두 읽지 않아도 괜찮습니다).

예를 들어, `"tutorial.kubebuilder.io/api/v1".CronJob{}` 타입을 `batch.tutorial.kubebuilder.io/v1` API 그룹에 속한 것으로 표시했다면(암시적으로 Kind가 `CronJob`임을 지정),

API 서버로부터 다음과 같은 JSON을 받을 때:

```json
{
  "kind": "CronJob",
  "apiVersion": "batch.tutorial.kubebuilder.io/v1",
  ...
}
```

나중에 `&CronJob{}`을 생성하거나 업데이트 시 올바른 그룹-버전을 조회할 수 있습니다.
