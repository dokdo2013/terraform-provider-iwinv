# 공유 스토리지 수명주기 결정

[English](../en/nas-lifecycle.md) · [아키텍처](architecture.md) · 2026-09-19

타입 어댑터는 NAS control-plane 5개 작업인 상품·전체 서비스 조회, 생성, 전체 권한 맵 교체, 삭제를 구현합니다.
`iwinv_shared_storage`는 T069 Core acceptance 후 등록했습니다. 상품 조회는 Terraform Data Source 없는 내부 어댑터입니다.

## 근거와 식별자

C19/C20/C21은 응답 스키마 부재와 입력 문서 불일치를 다룹니다. multipart 헤더 기본값과 달리 실제 요청은 JSON입니다.
문서의 허용 IP 배열은 생성·Read에서 IPv4 호스트→권한 object입니다. 수정 응답은 별도의 `ip`·`acl` 객체 배열이므로
중복 IP 없이 요청한 맵과 정확히 일치해야 합니다. 수정 응답만으로 완료하지 않고 정확한 ID를 다시 조회합니다.

`service_idx`는 부동소수점 반올림 없는 정확한 양의 int64를 사용합니다. 나머지 생성 응답을 검사하기 전에 ID를 확보·기록합니다.
Read는 서비스 배열 전체를 검증하며 오류·잘못된 행·중복 ID·페이지/건수 변경을 부재로 처리하지 않습니다.
조회 필드는 상품·별칭·설명 원문·상태·`spec.disk`·도메인·`mount_info`·전체 권한 맵입니다. 생성 응답에는 mount 정보가 없습니다.
mount 정보는 관측 문자열로 보존하며 실행할 명령으로 취급하지 않습니다. 타입 모델에서 인증정보와 임의 필드를 제외하고,
nullable 설명과 관측된 빈 맵을 보존합니다.

새 `api_nas` 실험은 pending 관측 후 조회 시작 7.12초에 active를 처음 확인했습니다. 요청한 100 GB·한글 설명 원문·초기 RO 맵이
일치했고 RW/RO를 포함한 전체 교체 후 삭제 접수·부재를 확인했습니다. 이 시간은 한 번의 관측이며 고정 대기나 지연 보장이 아닙니다.

## 용량, 생성, import

공식 생성 API는 100–2000 GB와 영문·숫자 6–20자의 공유 이름을 정의합니다. 어댑터는 정수 `hdd`와 정확한 `sharename`을 보냅니다.
카탈로그에는 디스크 범위가 있는 available `api_nas`와 빈 ID·0 범위·null 버전인 coming-soon 행이 있습니다.
행을 그대로 보존하며 빈 ID로 생성하지 않습니다. 문서화된 디스크 단위는 유지하되 가격은 검증 전 제외합니다.
상품 노출이 생성 자격을 보장하지 않으며 실환경 수명주기 근거는 available api_nas의 100·200 GB입니다.
이 control-plane에는 resize·이름·설명 수정 엔드포인트가 문서화되어 있지 않습니다.

`sharename`은 **Read에 없습니다.** 도메인이나 mount 문자열을 파싱해 생성 이력을 만들지 않습니다.
선택 입력 `share_name`은 생성 시 필수이며 Terraform으로 만든 객체에서만 이력을 유지하고 import 시 생략합니다.
이 이력의 추가·변경·제거 및 상품·이름·설명·용량 변경은 명시적으로 서비스를 교체해야 합니다.
정확한 서비스 ID로 import하면 모든 조회 필드를 복원하고 공유 이름 없이 무변경 plan을 얻어야 합니다.
교체에는 알려진 새 공유 이름을 요구합니다. NAS의 재사용 제약은 미검증이며 호스팅·캐시의 24시간 제한을 일반화하지 않습니다.

삭제는 저장소를 파괴하며 공식 문서상 복구할 수 없습니다. 교체는 제자리 resize나 데이터 이전이 아닙니다.
실제 데이터에는 `prevent_destroy`, 백업과 명시적인 이전 계획을 사용하세요. `create_before_destroy`는 새 공유를 먼저 만들 수 있지만
파일 복사나 클라이언트 설정 변경을 하지 않습니다. 합성 acceptance는 생성 실패 복구와 명시적인 taint/`-replace` plan을 검증하지만 같은 이름 재생성은 검증하지 않습니다.

## 권한 맵 소유와 대기

부모 리소스 하나가 정규 IPv4 호스트→정확한 `RO` 또는 `RW`의 **비어 있지 않은 맵 전체**를 소유해야 합니다.
수정은 전체 맵을 교체하여 누락한 호스트를 제거하고 남긴 호스트의 권한도 바꿉니다. 독립 리소스로 원소를 나누거나
여러 Terraform state가 같은 맵을 관리하면 안 됩니다. 이전 실험의 빈 PUT은 422로 거절됐고 IPv6/CIDR·다른 권한은 미지원입니다.
리소스는 이를 쓰기 전에 거절합니다. API 조회는 설정 검증이며 실제 NFS 권한 강제의 증거는 아닙니다.

생성은 ID를 보존하면서 active와 조회 가능한 설정 일치를 기다려야 합니다. pending/waiting은 context 기한 내에서 대기하며
알 수 없는 상태·조회 실패는 오류를 내야 합니다. 아직 검증된 적 없는 ID가 목록에 없더라도 refresh에서 보존하고 소유 ID 정리를
가능하게 해야 합니다. 수정은 전체 요청 맵이 반영될 때까지 기다리고 실패 시 이전 state를 유지하여 재확인합니다.
삭제는 접수와 검증된 정확한 ID 부재가 필요합니다. 쓰기는 자동 반복하지 않으며 캐시 전용 작업중 분류를 NAS에 적용하지 않습니다.
timeout·전송 실패·일반 404 이후에는 다음 쓰기 전에 실제 결과를 대조해야 합니다.

## 남은 acceptance

T069는 Terraform 스키마·unknown, import, 무변경 plan, drift와 교체를 다룹니다. 실패·timeout 경로는 합성 acceptance이며 실환경은 성공 경로를 검증했습니다.
NFS mount·파일 접근·tenant API 인증/작업·백업·스냅샷·이전·다른 상품·과금 종료는 control-plane 근거 범위 밖입니다.
전체 T038과 독립적으로 미해결인 웹메일 정리 T056은 미완료입니다.

출처: [생성](https://iwinv-api-nas.readme.io/reference/공유-스토리지-생성),
[허용 목록](https://iwinv-api-nas.readme.io/reference/접근-허용-ip-추가),
[삭제](https://iwinv-api-nas.readme.io/reference/공유-스토리지-삭제),
[상품](https://iwinv-api-nas.readme.io/reference/공유-스토리지-상품-조회).

## 타입 어댑터 acceptance (T068)

`TestAccNASControlPlaneWrites`는 Go race에서 25.82초로 통과했습니다. 새 100 GB api_nas 두 개가 active에 도달했고,
설명 원문과 생략을 확인했습니다. 전체 권한 맵 교체 후 기존 호스트의 RW를 RO로 바꾸고 다른 호스트를 제거했으며,
다른 서비스는 그대로였습니다. 둘 다 삭제 접수와 정확한 ID 부재를 확인했습니다. 앞선 실험을 포함해 새 ID 3개를 정리했고,
API NAS 콘솔에서도 검색 조건 없는 빈 목록을 독립 확인했습니다. 합성 검사는 잘못된 생성 응답 이후 ID 보존,
잘못된·부분 목록과 권한 응답 거절, 입력 검증과 자동 쓰기 반복 금지를 다룹니다. 카탈로그 범위·버전·nullability를
생성 자격과 별개로 검증합니다. 이 단계는 어댑터 범위의 근거이며 이후 Terraform acceptance는 아래에 기록합니다.

## Terraform acceptance (T069)

Core 실환경 acceptance가 82.29초로 통과했습니다. 100 GB 두 서비스, 권한 수정·drift, 모든 조회 속성 import,
공유 이름 이력 없는 영속 import·무변경 plan·수정, 새 공유 이름으로 200 GB 교체, 외부 삭제·재생성,
4개 ID의 삭제 접수·정확한 부재를 확인했습니다. 별도로 새로고침한 콘솔도 비어 있었습니다.
합성 검사는 unknown·잘못된 입력·실패/timeout·가시성 지연과 ID 보존·PUT 전 정확한 부모 검사를 다룹니다.
쓰기는 자동 반복하지 않습니다. taint/`-replace`는 plan만 검사했으며 같은 공유 이름 재사용은 보장하지 않습니다.
[사용자 가이드](../../docs/ko/resources/shared_storage.md)와 [상세 근거](contract-progress.md)를 참고하세요.
