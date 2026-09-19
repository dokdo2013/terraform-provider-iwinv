# 아키텍처와 Terraform 사용자 경험

[English](../en/architecture.md) · [목차](../../README.md) · 상태: 제안 · 리비전: 1

## 목표와 범위

공식 iwinv API·CLI의 리소스와 작업 전체를 단계적으로 지원하는 커뮤니티 Provider를 만듭니다.
지속되는 원격 객체 하나는 Terraform 리소스 하나에 대응합니다. 조회는 Data Source,
일회성 작업은 Action, 만료되는 인증정보·URL은 적절한 경우 Ephemeral Resource로 분류합니다.
로컬 CLI 설정은 클라우드 리소스가 아닙니다. API가 부족한 기능도 제외하지 않고 제약 대장에 남깁니다.

Go와 Terraform Plugin Framework, 공식 scaffolding을 사용합니다. CLI 실행·MCP 호출·콘솔 스크래핑·
`local-exec`로 Provider를 구현하지 않습니다. 지원 버전 조합은 P1에서 선정·검증합니다.
현재 개발 기준은 [ADR-0001](contract-progress.md)의 Terraform >=1.14.0입니다. Action·Ephemeral은 추가 기능별 검증을 거쳐야 합니다.

## AWS 스타일 인터페이스

| 익숙한 패턴 | iwinv 설계안 | 실제 API와의 관계 |
| --- | --- | --- |
| `aws_instance` | `iwinv_instance` | 서버 한 대; API 일괄 생성 count는 항상 1 |
| `instance_type` | `instance_type` | 표시 이름 추측 없이 정확한 `flavor_id` 사용 |
| `availability_zone` | `availability_zone` | 정확한 `zone_id`; AWS region/AZ 계층을 가정하지 않음 |
| AMI 참조 | `image_id` | iwinv 이미지 ID이므로 `ami`라는 이름은 사용하지 않음 |
| 키 페어 참조 | `ssh_key_ids` | 기존 키 ID의 집합; 정렬 후 쉼표 구분으로 전송 |
| 리소스 검색 | `iwinv_image`, `iwinv_instance_type`, `iwinv_availability_zones` | 지원 필터 명시, 전체 페이지 조회 |
| 독립 보안 규칙 | `iwinv_security_group_ingress_rule`, `iwinv_security_group_egress_rule` | 방향 값은 실측 후 정규화 |
| 독립 연결 | `iwinv_security_group_attachment` | 그룹과 서버의 관계; 연결 개수 제약 확인 필요 |
| EBS와 유사한 저장소 | `iwinv_block_storage` | 생성에 서버 ID 필수; 독립 볼륨 모델은 검증 보류 |
| Provider alias | `provider "iwinv" { alias = "secondary" }` | 자격증명·클라이언트·계정 캐시 분리 |
| 표준 수명주기 | `for_each`, `import`, `timeouts`, `prevent_destroy` | Terraform 기능이며 서버 측 보호 기능이 아님 |

API에서 확인되지 않은 `tags`, `default_tags`, ARN, IAM, VPC, subnet, `region`, inline `user_data`를 만들지 않습니다.
원격 이름은 `name`으로 표현합니다. User Script는 쓰기 API를 확인하기 전까지 기존 ID를 참조합니다.
`flavor_id`와 `instance_type`처럼 같은 의미의 입력을 이중 제공하지 않습니다.
AWS 구현을 복사하지 않고 사용자가 익숙한 구성·참조 방식을 iwinv 계약에 맞춥니다.

## 사용자 설정 예시: 설계안이며 실행 불가

```hcl
terraform {
  required_providers {
    iwinv = { source = "dokdo2013/iwinv" } # 제안 주소, 아직 게시되지 않음
  }
}

provider "iwinv" {} # 제안 환경변수: IWINV_ACCESS_KEY / IWINV_SECRET_KEY

variable "image_id" { type = string }
variable "instance_type" { type = string }
variable "availability_zone" { type = string }

resource "iwinv_instance" "web" {
  name              = "example-web"
  image_id          = var.image_id
  instance_type     = var.instance_type
  availability_zone = var.availability_zone

  timeouts {
    create = "30m" # 사용자 선택 예시이며 확정 기본값이 아님
    delete = "30m"
  }

  lifecycle { prevent_destroy = true }
}

output "server_id" { value = iwinv_instance.web.id }
```

실제 릴리스 이후 version 제약을 추가합니다. P2 통과 전에는 실행 가능한 quickstart로 안내하지 않습니다.
`prevent_destroy`는 설정 기반 보호입니다. 리소스 블록 자체를 지우면 이 보호도 사라지며 콘솔/API 삭제를 막지 않습니다.

## 인스턴스 스키마 제안

| 속성 | Terraform 형태 | 변경·import 정책 |
| --- | --- | --- |
| `id` | Computed string | 불투명한 API ID, 조회 시 안정적으로 유지 |
| `name` | Required string | 제자리 수정; 한글 글자/바이트 길이 실측 |
| `description` | Optional string | null·빈 문자열·삭제 의미 확인 후 기본값 선정 |
| `image_id` | Required string | 교체 제안; 기존 디스크를 조용히 rebuild하지 않음 |
| `instance_type` | Required string | 중단·실패 의미 검증 후 resize 지원; 미지원 중 변경은 명시적 오류 |
| `availability_zone` | Required string | 교체 제안; 실시간 이동 가능하다고 가정하지 않음 |
| `ssh_key_ids` | Optional set(string) | 생성 전용 제안; Read 미지원 시 제약 공개 |
| `user_script_id` | Optional string | 기존 스크립트 참조; 생성 전용·import 제약 동일 |
| `status` | Computed string | 서비스 상태 보존; 전원과 프로비저닝 상태를 혼동하지 않음 |
| `public_ip`, `private_ip` | Computed string | 검증된 기본 NIC 식별자로 선택, 배열 첫 원소 사용 금지 |
| `network_interfaces` | Computed collection | 복수 인터페이스 보존; 상세 형태 검증 후 확정 |
| `timeouts` | 표준 timeout 설정 | context와 연결, 측정값에 근거한 기본값 |

일반 Read/state에 기본 계정 비밀번호·VNC 토큰을 넣지 않습니다. 해당 값이 제외된 명시적인 필드 마스크를 요청합니다.
`Sensitive`는 표시를 가릴 뿐 state 저장을 막지 않습니다. 인증키는 Provider 설정에만 두며 리소스 state로 복사하지 않습니다.
비밀번호를 요구하는 서비스는 write-only 인자 적용 가능성과 state 노출 정책을 결정한 뒤 출시합니다.

## 상태·소유권·작업 모델

- 불변 ID로 import 후 원격 Read를 수행합니다. 규칙·연결은 명확한 부모/자식 복합 ID 형식을 문서화합니다.
  실제 값에 맞춘 설정으로 import 후 plan이 수렴해야 합니다.
- unknown·null·빈 값은 구분합니다. 광범위한 `UseStateForUnknown`, 무조건적인 diff 억제,
  ignore_changes로 변경 감지를 숨기지 않습니다.
- 조회할 수 없는 생성 입력은 ID만으로 복원할 수 없습니다. 명시적인 생성 전용 계약 아래 설정값을 보존하되,
  import된 객체의 알 수 없는 과거 값을 추측하지 않습니다. 이력 미확인과 빈 집합을 같은 값으로 만들지 않습니다.
- 그룹·규칙·연결은 각각 하나의 작성 주체만 가집니다. inline 규칙과 독립 규칙을 동시에 관리하지 않습니다.
  IP 허용 목록처럼 전체 교체 API라면 IP별 리소스 대신 집합 전체를 관리하는 단일 소유자를 사용합니다.
- 블록 스토리지는 생성 시 서버에 연결되는 계약입니다. 볼륨의 `instance_id`와 별도 attachment가 같은 연결을
  동시에 관리하지 않도록 이동·분리 계약이 확인될 때까지 소유 모델을 보류합니다.
- reboot·rebuild·캐시 purge·객체 copy/move·메시지 발송은 Action 후보입니다. 타임스탬프를 바꿔 실행하는 가짜
  리소스로 만들지 않습니다. 현재 Action은 리소스 state를 갱신하지 않으므로 rebuild 후 Read/설정 정합성은 별도 검증합니다.
- VNC·presigned URL은 Ephemeral 후보입니다. 접근 권한을 담은 URL을 일반 Data Source에 영구 저장하지 않습니다.
- refresh·Data Source는 변경을 일으키지 않습니다. 조회 API가 POST이더라도 이 원칙은 동일합니다.

## 구현 경계

`Terraform 스키마/수명주기 -> 서비스 어댑터 -> 타입이 있는 HTTP 클라이언트 -> 공식 API`

패키지는 `internal/provider`, `internal/services/{compute,storage,network,...}`, `internal/client`로 제안합니다.
인증·인코딩·페이지·오류·요청 제한·완료 대기를 독립 검증합니다. 초기 클라이언트는 같은 저장소에 두고,
재사용 수요와 호환성 계약이 생기면 공개 Go SDK로 분리합니다.
Control-plane HMAC, S3 서명, 메시징 인증, MCP OAuth는 별개입니다. 서비스별 자격증명 요구를 문서화합니다.

HTTPS 검증을 기본으로 하며 공식 예제의 TLS 검증 비활성화 코드를 그대로 따르지 않습니다.
endpoint override를 제공한다면 서비스별 명시적 설정으로 한정합니다. 다른 호스트로 redirect할 때 인증정보를 전달하지 않습니다.
시계를 주입해 서명 테스트를 하고, 대화형 로그인이나 공식 CLI 설정 파일에는 의존하지 않습니다.

## 오류·재시도·비동기 처리

매 시도마다 새 Timestamp로 서명합니다. HTTP 상태와 업무 오류를 함께 판정하고 안전한 진단만 제공합니다.
안전성이 확인된 조회만 제한된 backoff·jitter로 재시도하고 유효한 Retry-After를 존중합니다.
멱등성·복구 계약이 확인되지 않은 create/send/rebuild의 성공 여부가 불명확하면 자동 재전송하지 않습니다.
생성 ID를 받았다면 부분 실패 시에도 Framework가 허용하는 단계에서 state에 보존하고, 고아 리소스가 생기지 않는지 검증합니다.
응답 유실로 ID를 모르면 자동 채택하지 않고 조회·복구 안내와 함께 중단합니다.

context 기한 안에서 polling하며 pending·active·off·work·error와 생성 예시의 `building`을 구분합니다.
확정된 부재만 기존 state에서 제거합니다. 빈 페이지·잘못된 응답·401/403/429/5xx·예상 밖 202는 부재가 아닙니다.
생성 직후 404에는 제한된 유예가 필요합니다. 삭제 접수만으로 삭제·과금 종료가 완료되었다고 판단하지 않습니다.

요청과 polling을 함께 제한합니다. 동일 계정의 별도 프로세스·alias가 공유하는 quota는 로컬 limiter만으로 보장할 수 없어
429 처리와 전체 동시 실행 수 관리 안내도 필요합니다.

## 확정 조건

[API 계약](api-contract.md)과 [검증 계획](verification.md)을 통과한 뒤 공개 스키마를 확정합니다.
변경은 ADR로 기록하고 state migration과 변경 불가능한 semantic version 릴리스로 관리합니다.
근거는 [출처 대장](../sources.md)에 있습니다.

## 보안 그룹 리소스 경계 (개발용)

타입이 있는 네트워크 어댑터를 `iwinv_security_group`에 연결했습니다. 이름, 비어 있지 않은 설명과 ICMP 속성만 소유합니다.
실행 가능한 스키마와 정확한 ID import는 [리소스 가이드](../../docs/ko/resources/security_group.md)에 있습니다.
설명 응답은 HTML 디코딩을 한 번만 수행하고 이름은 그대로 유지합니다. 빈 설명 생성/초기화는 미지원이며,
설명 기본값은 `Managed by Terraform`입니다. 설명을 바꾸지 않는 수정에서는 해당 필드를 생략합니다.

생성은 나머지 응답/Read 검증 실패를 보고하기 전에 확보한 ID를 state에 기록합니다. 수정/삭제 실패는 이전 state를 보존합니다.
삭제는 성공 응답 후 상세 Read에서 부재를 확인합니다. API 오류로 state를 제거하거나 쓰기를 자동 재시도하지 않습니다.
작업별 timeout으로 성공했지만 반영이 덜 된 조회의 대기를 제한합니다. 규칙은 아래의 독립 리소스로 관리하며 서버 연결은 미구현입니다.
합성 및 실환경 Terraform 테스트로 import, 무변경 plan, drift와 외부 삭제를 검증했고, 합성 생성 실패 테스트로 Core의 ID 보존 및 정리를 확인했습니다.
이 리소스의 제한된 검증은 서버 단계 통과나 모든 네트워크 수명주기 계약 해결을 의미하지 않습니다.
검증 범위와 남은 공백은 [근거](contract-progress.md)를 참고하세요.

## 독립 규칙 리소스와 state 결정

`iwinv_security_group_ingress_rule`과 `iwinv_security_group_egress_rule`로 독립 규칙 설계를 구현했습니다.
`security_group_id`, `ip_protocol`, `from_port`, `to_port`, `cidr_ipv4`, 이름과 설명을 사용하며 인라인 소유권을 도입하지 않습니다.
원하는 방향이 고정된 공통 구현을 사용하되 계산 속성 `direction`에 실제 방향 drift를 기록하고 plan에서 명시적으로 복원합니다.
API가 수정 가능한 방향을 리소스 타입 이름 뒤에 숨기지 않도록 했습니다.

복합 식별자/import 형식은 `firewall_id/rule_id`이며 숫자 ID는 int64를 거쳐 정확한 십진수 문자열로 유지합니다.
규칙 title/content는 HTML escape하는 그룹 설명과 달리 원문을 유지합니다. 빈 설명 생성/null Read는 Terraform 빈 설명으로 표현합니다.
빈 값/null 수정이 기존 설명을 지우지 못하므로 설명 초기화는 교체합니다. 부모 변경도 교체합니다.
비어 있지 않은 기존 설명의 새 값이 plan에서 unknown이면 보수적으로 교체를 계획하며 apply에서 갑자기 교체를 추가하지 않습니다.
완전히 같은 규칙은 중복 생성이 거부되므로 create-before-destroy 성공을 가정하지 않습니다. 알려진 비어 있지 않은 설명은 제자리 수정합니다.

규칙 Read는 부모 상세를 먼저 검증한 뒤 페이지 없는 전체 규칙 목록을 읽습니다. 부모 부재는 엔드포인트별 성공 결과로만 판정하며
규칙 API 오류만으로 부재로 보지 않습니다. 개별 삭제는 다른 규칙을 보존하고 관리 부모 삭제 전에 완료합니다.
API 부재와 물리적인 연쇄 삭제/과금 종료는 구분합니다. [공통 규칙 가이드](../../docs/ko/guides/security_group_rules.md)와 T058 [근거](contract-progress.md)를 참고하세요.
