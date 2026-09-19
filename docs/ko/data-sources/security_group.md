---
page_title: "iwinv_security_group Data Source - iwinv"
subcategory: "Networking"
description: |-
  Read existing iwinv security group attributes.
---

# iwinv_security_group

[English](../../data-sources/security_group.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_security_group-data-source)

개발용 Provider이며 Registry 릴리스는 아직 없습니다. 정확한 기존 그룹 ID 한 개를 조회합니다. 이름 검색이나 첫 번째 결과 자동 선택은 하지 않습니다.

```hcl
variable "security_group_id" {
  type        = string
  description = "Exact existing iwinv FIREWALL ID."
}

data "iwinv_security_group" "selected" {
  id = var.security_group_id
}
```

`id`는 필수 string 입력이며 정확한 `FIREWALL-...` ID를 사용합니다. 0개·복수·다른 ID 응답은 오류입니다.

조회 속성:

| Attribute | Type | 의미 |
| --- | --- | --- |
| `name` | string | API 이름 원문, 중복 이름 가능. |
| `description` | nullable string | HTML escape를 한 번만 해제. null과 빈 문자열 구분. |
| `allow_icmp` | bool | API ICMP 플래그 Y/N에 대응. |

규칙·연결된 서버·임의 응답 필드는 포함하지 않습니다. 이 조회는 원격 객체를 소유하거나 수정·삭제하지 않으며 import 대상이 아닙니다. 속성 관리는 [보안 그룹 리소스](../resources/security_group.md)를 사용하세요.

정확한 ID의 상세 엔드포인트를 한 번 호출하며 그룹 목록을 순회하지 않습니다. 페이지 메타데이터 없는 HTTP 200 응답에서 count가 일치하고, 요청 ID와 같은 행이 정확히 하나이며 각 속성이 유효해야 합니다. 성공 응답이 빈 배열이면 찾을 수 없다는 진단을 반환하며 인증·HTTP 오류도 실패로 처리합니다. unknown ID 참조는 Terraform이 apply까지 미룹니다. 전체 페이지의 목록은 [security_groups](security_groups.md)를 사용하세요.

T073: 합성 Core에서 51행의 여러 페이지·순서 변경·같은 이름·null/빈 설명·정확한 ID·잘못된/없는 ID·권한/중간 오류·unknown 참조를 검증했습니다. 실환경은 이번 테스트에서 만든 그룹 하나의 목록/상세 일치, 설명 왕복, 외부 이름 수정 반영과 무변경 plan을 검증하고 해당 그룹을 삭제했습니다. 실환경 50개 초과 목록·연결·방화벽 트래픽 효력은 별도 검증입니다.

[Example](../../../examples/data-sources/iwinv_security_group/main.tf) · [Verification](../../../design/ko/contract-progress.md)

Sources: [official list](https://iwinv.readme.io/reference/get_v1-security-groups), [official detail](https://iwinv.readme.io/reference/get_v1-security-groups-id).
