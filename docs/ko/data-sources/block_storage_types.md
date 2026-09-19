---
page_title: "iwinv_block_storage_types Data Source - iwinv"
subcategory: "Storage"
description: |-
  블록 스토리지 타입의 용량 범위와 nullable 가용 존을 조회합니다.
---

# iwinv_block_storage_types

[English](../../data-sources/block_storage_types.md) · [전체 스키마 참조](../guides/schema_reference.md#iwinv_block_storage_types-data-source)

스토리지를 생성하지 않고 디스크 타입 카탈로그를 읽습니다. 개발용 Provider이며 Registry 릴리스는 아직 없습니다.

```hcl
data "iwinv_block_storage_types" "all" {}

data "iwinv_block_storage_types" "ssd" {
  type = "ssd"
}
```

`type`은 선택적인 정확한 문자열 필터입니다. 생략하거나 null이면 전체 카탈로그를 조회합니다. 빈 문자열은 API 호출 전에
거절하며, unknown 값은 전체 조회로 바꾸지 않고 apply까지 조회를 미룹니다. 타입 enum을 고정하거나 기본 타입을 자동 선택하지
않습니다. 지원하지 않는 코드의 실환경 조회는 빈 결과 대신 HTTP 400 `CHECK_PARAM` 오류였습니다.

`types`는 디스크 타입의 사전 순서로 정렬한 계산 객체 목록입니다.

| 속성 | 타입 | 의미 |
| --- | --- | --- |
| `type` | string | 정확한 API 디스크 타입. AWS 볼륨 타입과 임의로 대응하지 않습니다. |
| `minimum_size_gb` | int64 | 문서상 GB 단위의 최소 용량. 이진 단위로 환산하지 않습니다. |
| `maximum_size_gb` | int64 | 문서상 GB 단위의 최대 용량. resize 지원을 뜻하지 않습니다. |
| `availability_zones` | nullable list(string) | 정확한 존 코드를 사전 순서로 정렬. null과 빈 목록을 구분합니다. |

실측 카탈로그는 SSD 10–2000 GB와 null 존, SATA 10–20000 GB와 존 하나를 반환했습니다. null을 모든 존으로 해석하거나
계정의 존 카탈로그로 채우지 않습니다. 존 코드의 밑줄·하이픈도 그대로 보존합니다. 공식 예시의 SATA 존은 실측과 달랐으므로
문서 예시를 복사하지 말고 현재 응답을 확인하세요. 카탈로그에 보인다는 사실만으로 생성 권한·서버 호환성·연결/분리 안전성·가격을
판단할 수 없습니다. 테스트 계정의 Compute 생성이 제한돼 블록 스토리지 수명주기는 아직 미구현입니다.

페이지 없는 HTTP 200 응답의 배열과 정확한 count를 검증합니다. 타입·용량의 누락/null, 음수·역전 용량, 정수가 아니거나
int64를 넘는 값, 누락/잘못된 존 목록, 빈/null/중복 존, 중복 타입, 필터 불일치, 예상하지 못한 메타데이터와 API 오류는
조회 전체를 실패시킵니다. 정상 성공 빈 배열은 빈 목록이며 HTTP 오류를 빈 결과로 바꾸지 않습니다. 원격 소유권·import는 없고
임의 응답 필드는 제외합니다.

T074는 어댑터의 엄격한 디코딩, 2^53을 넘는 정확한 정수, null/빈 목록 구분, 타입/존 순서 변경, 필터·오류·unknown 값을
어댑터 단위 테스트와 Terraform Core 테스트로 검증합니다. 실환경은 전체·SSD·SATA 조회와 후속 무변경 plan을 확인하며, 별도 읽기 전용 probe에서 지원하지
않는 필터의 실패도 확인했습니다. 볼륨 생성·연결·resize·삭제 검증은 포함하지 않습니다.

[예제](../../../examples/data-sources/iwinv_block_storage_types/main.tf) · [근거](../../../design/ko/contract-progress.md)

출처: [공식 API](https://iwinv.readme.io/reference/getv1blockstoragestypes),
[공식 CLI 명령](https://docs.iwinv.kr/developers/cli/commands/block-storages).
