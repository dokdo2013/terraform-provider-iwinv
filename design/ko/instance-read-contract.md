# 인스턴스 조회 계약 준비

[English](../en/instance-read-contract.md) · [목차](../../README.md)

## 구현 경계

`internal/services/compute/instances.go`에 내부 목록·정확한 ID 조회를 추가했습니다. Terraform 인스턴스 리소스·Data Source는 등록하지 않았습니다. 공개 지원은 기존 Data Source 18개·리소스 7개입니다. P2의 Read 경계를 준비한 것으로, 막혀 있는 Compute 존 생성 계약을 우회하지 않습니다.

[목록 API](https://iwinv.readme.io/reference/getv1instances)는 API 지원 존의 서버만 보인다고 명시합니다. 빈 목록으로 콘솔의 서버가 삭제됐다고 판단하면 안 됩니다. [상세 API](https://iwinv.readme.io/reference/getv1instancesinstanceid)도 result 배열이므로 정확히 하나의 일치하는 ID를 요구합니다. 빈·복수·다른 ID 결과는 오류로 유지합니다. 인증·404·요청 제한·서버 오류를 부재로 바꾸거나 state를 제거하지 않습니다. 데이터가 있는 소유 fixture의 계약을 확인하기 전까지 import·관리 수명주기는 보류합니다.

## 선택한 조회 범위

2026-09-19 확인한 [공식 필드 표](https://api-kr.iwinv.kr/fields/v1/instances)에 따라 마스크를 **3599**로 고정했습니다.

| API 필드 | 비트 | 타입이 있는 결과 |
| --- | --- | --- |
| instance_id | 1 | ID |
| name | 2 | 빈 문자열을 보존하는 이름 |
| description | 4 | 필드가 존재하는 문자열 또는 null; HTML·Unicode 정규화 없음 |
| status | 8 | 비어 있지 않은 원래 상태; 준비 완료 상태를 추측하지 않음 |
| zone | 512 | zone_id |
| flavor | 1024 | flavor_id |
| image | 2048 | image_id |

`default_account`(128)와 `vnc`(16384)는 요청하지 않습니다. 서버가 보내더라도 타입이 있는 결과에 보관할 필드가 없습니다. 원본 응답·기본 비밀번호·콘솔 링크를 반환하지 않으며 나머지 중첩 메타데이터도 제외합니다. 선택된 ID의 누락·null·잘못된 구조는 기본값으로 보정하지 않고 거절합니다. 실제 데이터 검증 결과에 따라 등록 전에 이 후보 규칙을 수정해야 할 수 있습니다.

목록은 페이지당 10건으로 count·페이지 번호·크기를 확인하고 페이지 내부·사이의 중복 ID를 거절합니다. 짧은 페이지로 종료한 후 정렬된 전체 결과만 반환합니다. 마지막 페이지가 정확히 10건이면 다음 빈 페이지까지 조회합니다. 1,000페이지 한도와 context 취소를 적용하고 후속 오류는 부분 결과를 폐기합니다. 문서에 total·snapshot token이 없으므로 동시 변경 중 계정 전체의 일관된 스냅샷을 보장하지 않습니다. 필터는 아직 제공하지 않습니다.

## 검증 근거와 다음 조건

합성 테스트는 고정 마스크, 요청하지 않은 비밀 필드의 canary, Unicode·null·빈 문자열, 잘못된 선택 필드, 상세의 0개·정확한 1개·복수 결과, 중복·잘못된 후속 페이지, HTTP 상태 오류, 반복 한도, 취소 및 후속 전송 오류의 재시도 금지를 검증합니다. 테스트 식별자는 모두 가상 값입니다.

2026-09-19 공식 endpoint에서 `TestAccInstanceListRead`가 통과했고 **API에서 보이는 서버는 0건**이었습니다. 선택한 마스크의 인증·빈 페이지 메타데이터만 확인했습니다. 데이터가 있는 응답의 projection·필드 제외, 상세·없는 ID·실환경 페이지·수명주기 합격이 아닙니다. 클라우드 자원 생성·변경은 없었고 비공개 로그나 키를 공개 저장소에 넣지 않았습니다.

인증정보를 비공개 환경변수로 설정한 상태에서 다음 읽기 전용 검증을 실행합니다.

```sh
TF_ACC=1 IWINV_LIVE_READ=1 go test -race ./internal/services/compute -run '^TestAccInstanceListRead$' -count=1 -v
```

등록 전에는 Compute 이용 조건을 해결하고 소유 fixture 한 개를 만들어 선택 필드를 안전한 콘솔 정보와 대조해야 합니다. nullability·상세 ID·페이지·확정된 부재를 확인한 뒤 CRUD·import·state 복구와 한영 사용자 문서를 구현합니다. T008은 진행 중이며 합격이 아닙니다. C01/C06과 P2 조건도 열려 있습니다.
