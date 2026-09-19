---
page_title: "iwinv_ssh_key Data Source - iwinv"
subcategory: "Compute"
description: |-
  전체 SSH 키 목록을 검증한 뒤 정확한 ID로 기존 키 하나를 찾습니다.
---

# iwinv_ssh_key

[English](../../data-sources/ssh_key.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_ssh_key-data-source)

전체 SSH 키 목록을 검증한 뒤 정확한 ID로 기존 키 하나를 찾습니다.

개발 버전이며 Registry 릴리스는 아직 없습니다. [Provider 설정](../index.md)을 먼저 확인하세요.

## 사용 예제

```hcl
variable "ssh_key_id" {
  type = string
}

data "iwinv_ssh_key" "selected" {
  id = var.ssh_key_id
}

output "ssh_key_name" {
  value = data.iwinv_ssh_key.selected.name
}
```

## 입력 인자

`id` (필수 string): [ssh_keys](ssh_keys.md)에서 선택한 정확한 ssh_key_id입니다. Read 시점에 알려진 비어 있지 않은 값이어야 합니다. ID는 불투명한 문자열로 비교하며 추측한 상세 endpoint에 삽입하지 않습니다.

## 조회 속성

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `id` | `string` | 설정한 정확한 SSH 키 참조 ID입니다. |
| `name` | `string` | API 표시 이름이며 빈 이름도 그대로 보존합니다. |

`id`는 필수 입력이며 나머지는 계산 속성입니다.

## 동작과 제약

확인된 API가 목록 조회뿐이므로 앞 페이지에서 ID를 찾아도 모든 페이지 검증 후 반환합니다. 페이지·중복 ID·오류 규칙은 [ssh_keys](ssh_keys.md)와 같습니다. ID가 없으면 오류이며 중복 표시 이름으로 선택하지 않습니다. 키 내용을 반환하지 않으므로 분실한 개인키를 복구할 수 없습니다. 키를 import하거나 소유하지 않습니다. 실환경 조회·무변경 plan은 참조에 대한 검증이며 키 변경이나 실제 서버 SSH 로그인 검증은 아닙니다.

조회 전용이며 import는 해당하지 않습니다. API 오류를 빈 결과로 바꾸지 않습니다.

[Example](../../../examples/data-sources/iwinv_ssh_keys/main.tf) · [Evidence](../../../design/ko/contract-progress.md)

[공식 API](https://iwinv-common.readme.io/reference/get_new-endpoint-1-1)
