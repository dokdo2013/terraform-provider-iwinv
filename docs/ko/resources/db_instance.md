---
page_title: "iwinv_db_instance Resource - iwinv"
subcategory: "Database"
description: |-
  클라우드 DBMS 서비스와 전체 IPv4 허용 목록을 관리합니다.
---

# iwinv_db_instance (Resource)

[English](../../resources/db_instance.md) · [개발용 설치](../../../design/ko/development.md)

개발 단계의 제어 API 리소스이며 Registry 릴리스는 없습니다. 검증된 서비스 범위는 생성 가능한 STD Redis 상품입니다.
서비스 생성/삭제와 전체 허용 IP 목록을 관리합니다. DB 데이터·사용자/비밀번호·백업·복제·DNS·SQL/Redis 쿼리·엔진 업그레이드·
데이터 이전은 관리하지 않습니다. 다른 엔진/상품은 추가 acceptance가 필요합니다.

## 예제와 소유 범위

[전체 예제](../../../examples/resources/iwinv_db_instance/main.tf)에 검토한 상품 ID, 새 계정명과 허용할 IPv4 호스트 집합을 입력합니다.
서비스 삭제는 복구할 수 없으므로 예제에서 `prevent_destroy`를 사용합니다.

```hcl
resource "iwinv_db_instance" "example" {
  product_id   = var.product_id
  account_name = var.account_name
  name         = "tf-example-dbms"
  allowed_ips  = var.allowed_ips

  lifecycle { prevent_destroy = true }
}
```

API 이름은 “추가”이지만 `allowed_ips`는 **허용 목록 전체**를 소유합니다. 수정하면 이전 목록을 교체하고,
외부 변경은 drift로 감지해 설정된 집합으로 복구합니다. 순서 변경은 차이를 만들지 않습니다.
한 리소스가 모든 항목을 관리해야 하며 다른 리소스나 state로 같은 목록을 동시에 관리하지 마세요.

실환경 API는 빈 PUT을 거부하고 이전 값을 유지했습니다. 따라서 하나 이상의 정규 IPv4 호스트를 요구하며
빈 집합·CIDR·IPv6·null 항목은 쓰기 전에 거부합니다. 마지막 주소 제거는 지원하지 않습니다.
destroy는 허용 목록을 비우는 작업이 아니라 DBMS 자체 삭제입니다. 기존의 빈 관측 목록은 읽을 수 있지만 관리 설정은
지원되는 비어 있지 않은 집합이어야 합니다. 제어 API의 IP 설정 확인이 실제 패킷 필터링이나 DB 접속 성공까지 증명하지는 않습니다.

## 스키마

| 속성 | 타입 | 동작 |
| --- | --- | --- |
| `product_id` | 필수 string | 비어 있지 않은 정확한 생성 ID. 변경 시 교체. |
| `name` | 필수 string | 별칭, Unicode 4–32자. 변경 시 교체. |
| `account_name` | 선택 string | 영문 6–12자 초기 계정. 생성 필수, Read에 없음. 과거 입력의 추가/변경/제거 시 교체. |
| `description` | 선택 + 계산 string | 기본 빈 값, 최대 50자. 빈 값은 생성 시 생략. 변경 시 교체. |
| `allowed_ips` | 필수 set(string) | 비어 있지 않은 전체 IPv4 호스트 집합. 제자리 수정. |
| `id` | 계산 string | float64 반올림 없는 양의 정수 `service_idx` 문자열. |
| `status` | 계산 string | 제어 API 상태. `active`가 접속 성공을 보장하지 않음. |
| `product_type` | 계산 string | 관측한 `spec.type` 등급. 엔진 이름이 아님. |
| `engine_version` | 계산 string | 관측한 `spec.ver`. 설정 가능한 버전/업그레이드 선택값이 아님. |
| `address` | 계산 string | 기본 도메인. 포트나 프로토콜을 추측해 붙이지 않음. |
| `domains` | 계산 map(string) | 관측한 전체 라벨→도메인 맵. |

로컬 `timeouts` 기본값은 create/update/delete `5m`, read `1m`이며 양수 기간만 허용합니다.
타임아웃만 변경하면 원격 쓰기를 하지 않습니다. 이름/설명은 HTML 디코딩이나 정규화 없이 원문을 유지합니다.

카탈로그에는 버전 간 중복 상품 ID와, available인데 생성 ID가 빈 행도 있습니다(C30).
생성 API는 `product_id`만 받고 버전 선택값은 받지 않습니다. 빈 ID 선택, 버전 행의 임의 중복 제거,
특정 카탈로그 버전이 생성된다는 가정을 피하고 생성 후 관측 버전을 확인하세요.
CPU·가격·저장공간 단위 필드는 검증 전까지 제외합니다. DBMS 카탈로그 Data Source는 아직 등록하지 않았습니다.

## 교체와 import

검증된 수정 API는 허용 IP 목록뿐입니다. 이름·상품·설명·계정 이력을 변경하면 서비스 전체와 데이터가 교체됩니다.
일반 교체에는 확정된 다른 계정명을 요구합니다. import한 서비스에 계정 이력이 없다면 Read로 이전 계정을 확인할 수 없으므로
사용자가 새 이름을 명시적으로 선택해야 합니다. 백업과 데이터 이전을 계획하세요.
`create_before_destroy`는 새 계정을 먼저 생성할 수 있지만 데이터나 접속 애플리케이션을 이전해 주지는 않습니다.

명시적 `-replace`, taint, 외부 삭제는 별도 Terraform Core 동작이며 일반 속성 비교를 우회할 수 있습니다.
재생성에는 새 계정명을 선택하세요. DBMS 삭제 후 계정명 재사용 규칙은 미검증이며, 호스팅의 24시간 제한을 이 서비스에
가정하지 않습니다. 명시적 교체 테스트는 같은 이름을 실제로 재생성하지 않고 plan만 검증합니다.

읽을 수 있는 속성에 설정을 맞춘 뒤 정확한 서비스 ID로 import합니다.

```sh
terraform import iwinv_db_instance.example 123456789
terraform plan
```

위 ID는 합성 예시입니다. 생성 이력이 없으면 `account_name`을 생략하세요. 도메인에서 추측하지 않습니다.
Read는 상품·이름·설명·허용 IP·상태·등급/버전·도메인을 복구합니다. 초기 계정 이력과 로컬 타임아웃은 import되지 않습니다.
나중에 계정명을 추가하는 것은 원격 관측값 확인이 아니라 교체 요청입니다. 두 state가 한 서비스를 동시에 소유하면 안 됩니다.
가져온 허용 목록과 설명에 설정을 맞춰 무변경 plan을 확인하세요.

## 실패 복구

생성 요청은 한 번만 보내며 확실한 반환 ID를 나머지 응답 검증 전에 저장한 뒤 관리 속성이 일치하는 active Read를 기다립니다.
생성 응답의 도메인은 문자열, Read의 도메인은 객체이므로 별도 디코더를 사용합니다.
미검증 응답과 목록 노출 지연에도 ID를 유지합니다. 아직 검증되지 않은 생성이 목록에 없으면 Read 오류를 내고 ID를 보존하며,
삭제는 알려진 ID에 한 번 요청합니다. 과거 active였던 서비스의 확정된 부재는 state에서 제거할 수 있습니다.

수정 오류·불일치 응답·수렴 대기 만료 시 이전 state를 유지합니다. 원격 결과가 달라졌을 수 있으므로 다음 쓰기 전에 refresh로 확인하세요.
API 오류·HTTP 404·부분 목록·중복·바뀐 페이지 메타데이터는 부재가 아닙니다. 쓰기를 자동 반복하지 않습니다.
삭제 응답과 정확한 ID 부재를 모두 요구하며, 이것만으로 과금 종료를 독립적으로 확인했다고 보지 않습니다.

[수명주기 결정](../../../design/ko/dbms-lifecycle.md)과 [검증 근거](../../../design/ko/contract-progress.md)를 참고하세요.
별도 웹메일의 정리 실패는 계속 열려 있으며 DBMS 검증 범위를 전체 정리로 확대하지 않습니다.

출처: [생성](https://iwinv-dbms.readme.io/reference/클라우드-dbms-생성),
[허용 IP](https://iwinv-dbms.readme.io/reference/접근-허용-ip-추가),
[삭제](https://iwinv-dbms.readme.io/reference/클라우드-dbms-삭제),
[상품](https://iwinv-dbms.readme.io/reference/클라우드-dbms-상품-조회).
