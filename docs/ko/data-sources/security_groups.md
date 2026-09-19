---
page_title: "iwinv_security_groups Data Source - iwinv"
subcategory: "Networking"
description: |-
  Read existing iwinv security group attributes.
---

# iwinv_security_groups

[English](../../data-sources/security_groups.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_security_groups-data-source)

개발용 Provider이며 Registry 릴리스는 아직 없습니다. 계정에서 API로 보이는 그룹 전체를 조회합니다. 입력 인자나 이름 필터는 없습니다.

```hcl
data "iwinv_security_groups" "all" {}
```

`ids`는 계산된 `list(string)`, `groups`는 계산된 객체 목록입니다. 두 목록 모두 ID의 사전 순서이며 같은 위치는 같은 그룹입니다. 빈 계정은 빈 목록입니다.

조회 속성 (`groups` 각 행):

| Attribute | Type | 의미 |
| --- | --- | --- |
| `id` | string | 정확한 원격 ID. |
| `name` | string | API 이름 원문, 중복 이름 가능. |
| `description` | nullable string | HTML escape를 한 번만 해제. null과 빈 문자열 구분. |
| `allow_icmp` | bool | API ICMP 플래그 Y/N에 대응. |

규칙·연결된 서버·임의 응답 필드는 포함하지 않습니다. 이 조회는 원격 객체를 소유하거나 수정·삭제하지 않으며 import 대상이 아닙니다. 속성 관리는 [보안 그룹 리소스](../resources/security_group.md)를 사용하세요.

목록은 모든 페이지의 개수·페이지 번호·크기·필드·ID 중복을 검증한 후 반환합니다. 인증·HTTP 오류, 중간 페이지 실패를 빈 결과나 부분 결과로 바꾸지 않습니다. 정렬은 응답 순서만 안정화하며 페이지 사이에 다른 사용자가 목록을 바꾸는 경우 원자적인 스냅샷을 보장하지 않습니다. ID 입력은 없습니다. 정확한 ID 하나의 상세 조회에는 [security_group](security_group.md)을 사용하세요.

T073: 합성 Core에서 51행의 여러 페이지·순서 변경·같은 이름·null/빈 설명·정확한 ID·잘못된/없는 ID·권한/중간 오류·unknown 참조를 검증했습니다. 실환경은 이번 테스트에서 만든 그룹 하나의 목록/상세 일치, 설명 왕복, 외부 이름 수정 반영과 무변경 plan을 검증하고 해당 그룹을 삭제했습니다. 실환경 50개 초과 목록·연결·방화벽 트래픽 효력은 별도 검증입니다.

[Example](../../../examples/data-sources/iwinv_security_groups/main.tf) · [Verification](../../../design/ko/contract-progress.md)

Sources: [official list](https://iwinv.readme.io/reference/get_v1-security-groups), [official detail](https://iwinv.readme.io/reference/get_v1-security-groups-id).
