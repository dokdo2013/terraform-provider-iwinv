# iwinv_security_group_ingress_rule 리소스

[English](../../resources/security_group_ingress_rule.md) · [설치](../../../design/ko/development.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_security_group_ingress_rule-resource)

ingress 규칙 하나를 관리하는 개발용 리소스입니다. 해당 규칙의 속성만 소유하고 부모 그룹이나 다른 규칙은 소유하지 않습니다.
아직 Registry 릴리스는 없습니다. 미연결 전용 그룹에서 control-plane 수명주기를 검증했으며 실제 패킷 필터링은 미검증입니다.

## 예제

```hcl
resource "iwinv_security_group" "example" {
  name = "tf-example-group"
}

resource "iwinv_security_group_ingress_rule" "example" {
  security_group_id = iwinv_security_group.example.id
  name              = "example-ingress"
  description       = "Example rule"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = "192.0.2.0/24"
}
```

[실행 가능한 전체 개발용 예제](../../../examples/resources/iwinv_security_group_ingress_rule/main.tf).
부모 참조를 사용하면 Terraform이 의존성을 인식하여 관리 규칙을 먼저 삭제하고 그룹을 삭제합니다.

## 스키마

| 속성 | 타입 | 동작 |
| --- | --- | --- |
| `security_group_id` | string, 필수 | 정확한 부모 `firewall_id`. 변경하면 규칙 교체. |
| `name` | string, 필수 | Unicode 코드 포인트 1–25개. 제자리 수정. |
| `description` | string, 선택 + 계산 | 기본값 빈 문자열, 최대 25 코드 포인트. 비어 있지 않은 값을 비우면 교체. |
| `ip_protocol` | string, 필수 | `tcp` 또는 `udp`. 제자리 수정. |
| `from_port`, `to_port` | int64, 필수 | 양 끝 포함 1–65535, `to_port >= from_port`. 같으면 단일 포트. 제자리 수정. |
| `cidr_ipv4` | string, 필수 | IPv4 CIDR. 호스트 비트를 그대로 보존. 제자리 수정. |
| `id` | string, 계산 | `security_group_id/rule_id` 복합 식별자. |
| `rule_id` | string, 계산 | 부동소수점 변환 없이 보존한 정확한 양의 십진수 API 규칙 ID. |
| `direction` | string, 계산 | 관측된 방향. 리소스 타입이 원하는 방향을 결정. |

선택 `timeouts` 블록의 `create`, `update`, `delete` 기본값은 `5m`, `read`는 `1m`입니다. 양수 duration 문자열을 사용하세요.
태그, IPv6, ICMP 규칙 프로토콜, 다른 보안 그룹을 source로 참조하는 기능은 제공하지 않습니다. ICMP는 별도 부모 그룹 플래그입니다.
API는 포트 0을 거부합니다. 이름·설명의 HTML 엔티티는 원문으로 유지하며, API가 null로 반환하는 빈 설명은 Terraform의 빈 문자열로 표현합니다.
멀티바이트 길이 경계와 패킷 동작을 모두 실측한 것은 아닙니다.

## 가져오기와 수명주기

```sh
terraform import iwinv_security_group_ingress_rule.example FIREWALL-REPLACE_WITH_YOUR_ID/123
terraform plan
```

위 두 ID는 자리표시자입니다. 정확한 부모와 규칙 ID를 사용하고 가져온 속성과 일치하는 설정을 작성하세요.
import는 빈 설명을 포함한 지원 속성을 복원합니다. 로컬 timeout 설정은 가져오지 않습니다.
방향이 다른 리소스에 import하면 plan에 방향 수정이 표시됩니다. 적용 전에 올바른 리소스 타입을 선택하세요.

외부에서 방향이 바뀌면 plan에 차이를 표시하고 제자리 수정으로 `ingress`를 복원합니다.
비어 있지 않은 설명을 지정하면 제자리 수정합니다. 설명을 비우거나 기존 비어 있지 않은 인수를 제거하거나 부모를 바꾸면 교체합니다.
이전 설명이 비어 있지 않은데 새 설명이 plan 단계에서 unknown이면 교체를 보수적으로 계획합니다.
apply에서 갑자기 교체가 추가되지 않도록 하기 위한 정책이며, 제자리 수정을 원하면 plan에서 알 수 있는 설명 값을 사용하세요.

교체·import·정리 전에 공통 [규칙 수명주기와 복구 가이드](../guides/security_group_rules.md)를 확인하세요.
검증: T058, [실환경 및 합성 근거](../../../design/ko/contract-progress.md).
