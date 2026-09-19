---
page_title: "iwinv_ssh_keys Data Source - iwinv"
subcategory: "Compute"
description: |-
  키 내용을 노출하거나 키를 변경하지 않고 기존 SSH 키의 참조 목록을 조회합니다.
---

# iwinv_ssh_keys

[English](../../data-sources/ssh_keys.md)

키 내용을 노출하거나 키를 변경하지 않고 기존 SSH 키의 참조 목록을 조회합니다.

개발 버전이며 Registry 릴리스는 아직 없습니다. [Provider 설정](../index.md)을 먼저 확인하세요.

## 사용 예제

```hcl
data "iwinv_ssh_keys" "available" {}

output "available_ssh_keys" {
  value = data.iwinv_ssh_keys.available.keys
}
```

## 입력 인자

서비스별 입력 인자는 없습니다. 이름 필터는 없습니다.

## 조회 속성

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `ids` | `list(string)` | 정확한 ssh_key_id를 사전순으로 정렬합니다. |
| `keys` | `list(object)` | 각 키의 문자열 id·name이며 ids와 같은 순서입니다. |

모든 속성은 계산 속성이므로 설정에 값을 지정하지 않습니다.

## 동작과 제약

10개씩 모든 페이지를 검증하고 짧은 페이지에서 끝납니다. 필요하면 마지막 빈 페이지도 읽습니다. 이름 누락/null, 빈 ID, 중복 ID, 변경되거나 잘못된 페이지 메타데이터, 중간 페이지 오류는 전체 조회 실패입니다. 빈 이름과 중복 표시 이름은 보존하며 식별자로 사용하지 않습니다. 최대 1,000페이지입니다. 정상 빈 배열은 빈 ids/keys를 반환합니다. 공개키·개인키·지문·다운로드 등 임의 응답 필드는 제외합니다. 정확한 ID 선택에는 [ssh_key](ssh_key.md)를 사용하세요. 키 생성·업로드·삭제와 서버의 SSH 키 설치는 수행하거나 검증하지 않습니다.

조회 전용이며 import는 해당하지 않습니다. API 오류를 빈 결과로 바꾸지 않습니다.

[Example](../../../examples/data-sources/iwinv_ssh_keys/main.tf) · [Evidence](../../../design/ko/contract-progress.md)

[공식 API](https://iwinv-common.readme.io/reference/get_new-endpoint-1-1)
