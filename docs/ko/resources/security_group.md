# iwinv_security_group 리소스

[English](../../resources/security_group.md) · [개발용 설치](../../../design/ko/development.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_security_group-resource)

보안 그룹의 속성을 관리하는 개발용 리소스입니다. 아직 Registry 릴리스는 없습니다.
정확한 `firewall_id` 하나의 `name`, `description`, `allow_icmp`를 소유합니다.
인라인 규칙과 서버 연결은 관리하지 않습니다. 규칙은 [독립 리소스](../guides/security_group_rules.md)를 사용하며 연결 리소스는 아직 구현하지 않았습니다.
실환경 수명주기 검증은 서버에 연결하지 않은 전용 그룹에서 수행했습니다. 통합 테스트에서는 독립 관리 규칙을 부모보다 먼저 삭제합니다. 패킷 필터링, 연결된 그룹 삭제와 물리적인 규칙 연쇄 삭제는 미검증입니다.
이 범위에 맞는 전용 그룹을 사용하세요. destroy 전에 의존성을 확인하고, 부모 삭제 시 관리 대상 밖의 자식 객체가 보존된다고 가정하지 마세요.

## 예제

```hcl
resource "iwinv_security_group" "example" {
  name        = "tf-example-group"
  description = "Managed by Terraform"
  allow_icmp  = false

  timeouts {
    create = "5m"
    read   = "1m"
    update = "5m"
    delete = "5m"
  }
}
```

[전체 예제](../../../examples/resources/iwinv_security_group/main.tf)를 실행하려면 개발용 설치와 환경변수 인증정보가 필요합니다.
`apply`는 실제 클라우드 객체를 만들고 `destroy`는 삭제합니다. state는 공개 Git 밖에 보관하고 적용 전에 plan을 검토하세요.

## 스키마

| 속성 | 타입 | 동작 |
| --- | --- | --- |
| `name` | string, 필수 | 문서상 제한에 따라 Unicode 코드 포인트 4–32개. 제자리 수정. |
| `description` | string, 선택 + 계산 | Unicode 코드 포인트 1–50개. 기본값 `Managed by Terraform`. 제자리 수정. |
| `allow_icmp` | bool, 선택 + 계산 | 기본값 `false`. API에 `Y`/`N`을 명시적으로 전송. |
| `id` | string, 계산 | 정확한 API `firewall_id`. 지원되는 수정 동안 유지. |

멀티바이트 문자열의 API 길이 경계를 모두 실측한 것은 아닙니다. 공백 제거, Unicode 정규화, URL 인코딩을 하지 않습니다.
실측 API는 설명을 HTML escape하므로 Provider가 한 번만 디코딩하며, 이름은 그대로 유지합니다.
`&amp;`처럼 사용자가 적은 리터럴 엔티티는 설정과 state에서도 리터럴로 유지됩니다.
명시적인 빈 설명은 apply 전에 거부합니다. 인수를 제거하면 기본값으로 되돌아가며 원격 설명을 비우지 않습니다.
이름/ICMP만 변경하고 설명을 유지할 때는 API 요청의 `content`를 생략합니다.

선택 `timeouts` 블록: `create`, `update`, `delete` 기본값은 `5m`, `read`는 `1m`입니다.
양수 duration 문자열을 사용하세요. 로컬 timeout 설정만 바꾸면 원격 update 요청을 보내지 않습니다.
제한 시간은 폴링과 클라이언트 요청 간격 대기를 포함한 전체 작업에 적용되며, 이미 진행한 원격 쓰기를 취소하지는 않습니다.

## 가져오기

기존 지원 속성과 일치하는 설정을 작성하고 정확한 `firewall_id`로 가져옵니다.

```sh
terraform import iwinv_security_group.example FIREWALL-REPLACE_WITH_YOUR_ID
terraform plan
```

위 ID는 자리표시자입니다. 이름으로 가져오지 않습니다. ID는 `FIREWALL-` 뒤에 영문자·숫자·`_`·`-`만 있는 단일 경로 구간이어야 합니다.
import는 세 속성을 조회하며 그룹을 생성하거나 규칙·연결의 소유권을 가져오지 않습니다.
일치하는 설정에서는 plan이 비어 있습니다. 로컬 timeout은 원격 값이 없으므로 가져오지 않습니다.
설명이 null/빈 문자열인 기존 그룹도 읽을 수 있지만, 빈 설명을 설정하는 기능은 지원하지 않습니다. 기본값이나 비어 있지 않은 값으로 수정 계획이 표시됩니다.
같은 그룹을 두 state에서 동시에 관리하지 마세요.

## 실패와 복구

쓰기 요청은 자동 재시도하지 않습니다. 생성 응답에서 ID 하나를 명확히 확보하면 나머지 응답 검증이나 Read 대기보다 먼저 state에 기록합니다.
그 이후 생성이 실패하면 Terraform은 해당 ID를 tainted로 표시하고 교체를 제안할 수 있습니다. 다음 apply 전에 실제 객체와 plan을 확인하세요.
기존 객체가 의도한 설정과 일치하는지 확인한 후 Terraform의 문서화된 복구 절차에 따라 유지하거나 명시적으로 삭제하세요.
ID를 받지 못했다면 비공개 요청 기록과 계정 목록을 먼저 대조하세요. Provider가 동명이인 객체를 채택하거나 생성 요청을 무작정 반복하지 않습니다.

생성/수정은 정확한 ID의 상세 조회 결과가 계획한 속성과 일치하면 끝납니다. 성공했지만 반영이 덜 된 조회만 폴링합니다.
API 오류, HTTP 404, 잘못된 응답과 예상 밖 ID는 오류로 남기며 state를 지우지 않습니다.
이 엔드포인트에서 실측한 HTTP 200 + 빈 배열 + count 0만 부재로 인정합니다.
삭제는 먼저 조회하고 DELETE 한 번을 보낸 후 부재를 기다립니다. 성공 응답만으로 완료하지 않습니다.
수정/삭제 실패 시 이전 state를 유지하여 후속 refresh로 실제 결과를 확인할 수 있습니다.
API 부재는 독립적인 과금 종료 확인과 같지 않습니다.

## 검증과 한계

합성 Terraform CLI 테스트는 수명주기, import, drift, timeout만 변경하는 경우와 생성 실패 후 ID를 보존한 정리를 검증합니다.
별도 opt-in 실환경 테스트는 생성/조회/수정, 전체 속성을 비교한 정확한 ID import, 무변경 plan, 외부 변경 복원, 외부 삭제/재생성과 최종 정리를 검증합니다.
[검증 근거](../../../design/ko/contract-progress.md)와 [실행 방법](../../../design/ko/development.md)을 참고하세요.
빈 그룹 설명 생성/초기화, 실제 계정의 전체 페이지 경계, 규칙의 패킷 동작, 연결과 서버 사용 가능 여부는 별도의 미해결 계약입니다.

출처: 공식 [생성](https://iwinv.readme.io/reference/post_v1-security-groups), [상세](https://iwinv.readme.io/reference/get_v1-security-groups-id), [수정](https://iwinv.readme.io/reference/put_v1-security-groups-id), [삭제](https://iwinv.readme.io/reference/delete_v1-security-groups-id), [Terraform 생성 state 규칙](https://developer.hashicorp.com/terraform/plugin/framework/resources/create).
