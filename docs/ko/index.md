---
page_title: "iwinv Provider"
description: |-
  독립 커뮤니티 iwinv Terraform Provider의 설정과 시작 방법입니다.
---

# iwinv Provider

[English](../index.md)

Terraform 설정·참조·import·변경 감지로 지원되는 iwinv control-plane 리소스를 관리합니다. 스마일서브/iwinv 공식 제품이 아닌 독립 커뮤니티 프로젝트입니다. 현재 개발 버전은 Data Source 18개와 리소스 7개를 구현합니다. **Registry에 게시한 버전은 아직 없습니다.** 예제 실행 전 [개발용 설치 안내](../../design/ko/development.md)를 따라 주세요. 아래 source 주소는 Provider 식별자이며 현재 다운로드 가능한 릴리스를 뜻하지 않습니다.

## 사용 예제

처음에는 조회 전용 존 목록으로 연결을 확인하세요. 서버를 생성하거나 계정을 변경하지 않습니다.

```hcl
terraform {
  required_version = ">= 1.14.0"
  required_providers {
    iwinv = {
      source = "dokdo2013/iwinv"
    }
  }
}

provider "iwinv" {}

data "iwinv_availability_zones" "available" {}

output "zone_ids" {
  value = data.iwinv_availability_zones.available.zone_ids
}
```

비공개 환경변수나 비밀정보 관리자를 통해 `IWINV_ACCESS_KEY`, `IWINV_SECRET_KEY`를 주입하세요. 콘솔 로그인·SSH·S3 키·MCP OAuth 토큰과 별개인 control-plane API 키입니다. 실제 키를 `.tf`·예제·셸 기록·이슈에 붙여넣지 마세요. 키의 허용 IP가 Terraform 실행 환경의 출발 IP와 맞아야 합니다. Provider는 고정 HTTPS endpoint `https://api-kr.iwinv.kr`에 TLS 검증으로 연결하며 CLI profile을 읽거나 대화형 로그인하지 않습니다.

개발 override를 설정한 뒤 `terraform validate`, `terraform plan` 순서로 실행하세요. validate에는 키가 필요 없고 plan의 실제 조회에는 키가 필요합니다. 이 override 설치 방식에서는 Registry `init`을 생략합니다. [릴리스 준비](../../design/ko/release-readiness.md)에는 서명 없는 filesystem mirror 설치와 일회용 테스트 키의 서명을 검증한 뒤 mirror로 설치하는 절차가 있습니다. 후자는 산출물 서명을 별도로 검사하며 Terraform의 mirror 설치가 GPG 서명을 인증하는 것은 아닙니다. 두 검증 모두 운영 서명키의 신뢰나 서명된 Registry 설치를 확인한 것은 아닙니다.

## 설정 인자

| 인자 | 타입 | 설정 방식 |
| --- | --- | --- |
| `access_key` | optional, sensitive string | 명시한 값이 `IWINV_ACCESS_KEY`보다 우선합니다. 생략/null이면 환경변수를 사용합니다. |
| `secret_key` | optional, sensitive string | 명시한 값이 `IWINV_SECRET_KEY`보다 우선합니다. 생략/null이면 환경변수를 사용합니다. |

Provider 구성 시 두 값이 모두 알려져 있고 공백 외 문자가 있어야 하며 줄바꿈을 포함하면 안 됩니다. 입력을 그대로 사용하므로 앞뒤 공백을 자동으로 제거하지 않습니다. 명시적인 빈 값이나 unknown은 오류이며 다른 계정의 환경변수로 전환하지 않습니다. 두 인자를 각각 해석하므로 다른 계정의 alias를 사용할 때는 의도한 같은 계정의 키 **두 개 모두** 지정하세요. Terraform 표준 `alias`, `provider = iwinv.alias_name`으로 설정을 선택할 수 있습니다. `region`, endpoint override, `profile`, `default_tags` 설정은 구현하지 않았습니다.

`Sensitive`는 일반 CLI 표시를 가리는 기능이며 암호화가 아닙니다. 임의의 Terraform 변수·출력·저장 plan을 공개해도 안전하다는 뜻도 아닙니다. 키는 환경변수 주입을 권장합니다. 리소스 비밀번호와 청구 state의 민감정보 정책은 해당 문서를 확인하세요.

## 요청 간격·시간 제한·복구

클라이언트는 조회와 폴링을 포함해 **Provider 설정 하나당** 요청 시작을 1초 간격으로 조절합니다. 이는 로컬 정책이며 검증된 계정 전체 quota가 아닙니다. 같은 계정을 쓰더라도 alias와 별도 Terraform 프로세스는 독립 클라이언트입니다. 동시 실행을 조율하세요. alias를 추가한다고 계정의 허용 API 호출량이 늘어나지는 않습니다.

각 HTTP 요청에는 고정 30초 제한이 있습니다. 리소스의 `timeouts`는 요청 차례 대기·HTTP 호출·반영 확인 폴링을 포함한 전체 작업을 제한합니다. 현재 리소스 기본값은 `read = "1m"`, `create`/`update`/`delete = "5m"`입니다. 리소스 제한을 늘려도 개별 HTTP 요청의 30초 제한은 늘어나지 않습니다. Data Source의 카탈로그 순회는 여러 번 요청할 수 있고 설정 가능한 `timeouts` 블록이 없으므로 HTTP 제한을 전체 조회의 제한 시간으로 해석하면 안 됩니다.

공통 전송 계층은 HTTP/API 오류·redirect·결과가 불확실한 쓰기를 자동 재시도하지 않습니다. [캐시 리소스](resources/content_cache.md)에는 검증된 작업중 거절 응답에 한정한 예외가 있으므로 해당 복구 규칙을 확인하세요. timeout이나 apply 중단은 iwinv가 이미 접수한 요청을 되돌리지 않습니다. state를 보존하고 정확한 리소스를 확인한 뒤 다시 적용하세요. 시간이 초과됐다는 이유만으로 state를 지우거나 생성을 반복하지 마세요.

## 필요한 기능 찾기

| 할 일 | 시작 문서 |
| --- | --- |
| API에 보이는 존 확인 | [availability_zones](data-sources/availability_zones.md) |
| 정확한 ID로 이미지 선택 | [images](data-sources/images.md) → [image](data-sources/image.md) |
| 정확한 ID로 서버 상품 선택 | [instance_types](data-sources/instance_types.md) → [instance_type](data-sources/instance_type.md) |
| 기존 SSH 키 참조 | [ssh_keys](data-sources/ssh_keys.md) → [ssh_key](data-sources/ssh_key.md) |
| 보안 그룹·독립 규칙 관리 | [security_group](resources/security_group.md), [규칙 가이드](guides/security_group_rules.md) |
| 호스팅 서비스 관리 | [webhosting](resources/webhosting.md), [db_instance](resources/db_instance.md), [content_cache](resources/content_cache.md), [shared_storage](resources/shared_storage.md) |
| 청구 요약 확인 | [current_bill](data-sources/current_bill.md), [bills](data-sources/bills.md) |

Data Source는 조회만 하며 원격 객체를 소유하거나 import하지 않습니다. 리소스는 서비스를 생성·수정·삭제하고 비용을 발생시킬 수 있습니다. 적용 전 해당 문서의 import·교체·비밀정보·삭제 제약을 읽으세요. 전용 테스트 리소스를 사용하고 정확한 생성 ID로 정리를 확인하세요.

인스턴스·연결·웹메일 서비스 및 메일 계정·오브젝트와 메시징 서비스 작업 등 전체 로드맵은 미완료입니다. 카탈로그에 보인다고 생성 자격·용량·호환성을 보장하지 않습니다. AWS에 익숙한 명명법을 사용하되 iwinv에 없는 IAM·ARN·태그 모델을 가정하지 않습니다. [기능 대장](../../design/inventory/implementation.json)과 [검증 기록](../../design/ko/contract-progress.md)에서 구현·설계안·미해결 계약을 구분합니다.

## 문제 해결

- **설정 오류:** Terraform 프로세스에 의도한 control-plane 키 두 개가 전달되는지, 명시적인 빈 값이나 unknown이 있는지 확인하세요. 진단을 위해 키를 출력하지 마세요.
- **IP·인증 오류:** 키의 출발 IP 정책과 실행 환경의 시계를 확인하세요. 다른 endpoint가 동작해도 특정 endpoint의 접근 제한은 남을 수 있습니다. 이를 빈 목록으로 해석하지 마세요.
- **정확한 ID가 없음:** 표시 이름·AWS 형식으로 변환한 ID·목록 순번 대신 해당 계정에서 조회한 ID를 선택하세요. 상세 조회 실패가 새 객체 생성 지시는 아닙니다.
- **계약·메타데이터 오류:** 비공개 근거를 보관하고 마스킹한 요약으로 제보하세요. 잘못된 응답이나 중간 페이지 실패를 부분 목록으로 처리하지 않습니다.

[iwinv 개발자 문서](https://docs.iwinv.kr/developers/cli/commands/account) · [커뮤니티 이슈](https://github.com/dokdo2013/terraform-provider-iwinv/issues). 키·state·저장 plan·계정 원응답·재무 정보를 제보에 포함하지 마세요.

전체 속성·중첩 구조·입력 및 비밀값 표시는 [스키마 참조](guides/schema_reference.md)에서 확인할 수 있습니다.
