---
page_title: "보안 그룹 규칙의 수명주기와 복구"
subcategory: ""
description: |-
  보안 그룹 규칙의 소유권, 수정, 교체, import와 실패 복구를 설명합니다.
---

# 보안 그룹 규칙의 수명주기와 복구

[English](../../guides/security_group_rules.md) · [Ingress](../resources/security_group_ingress_rule.md) · [Egress](../resources/security_group_egress_rule.md)

리소스 하나가 정확한 부모 그룹 안의 숫자 규칙 ID 하나를 소유합니다. 같은 조건이나 이름의 다른 규칙을 자동 채택하지 않습니다.
부모 리소스는 그룹 속성만 관리하고, 독립 규칙 리소스만 해당 규칙의 속성을 작성합니다.
참조로 의존성을 표현하고 각 규칙은 하나의 state에서 관리하세요. 관리 규칙이 남은 부모를 수동으로 삭제하지 않는 것이 좋습니다.

## 수정과 교체

프로토콜(`tcp`/`udp`), 포트 범위, IPv4 CIDR, 이름, 비어 있지 않은 설명은 ID를 유지한 채 수정합니다.
API가 방향 수정을 지원하므로 관측한 방향 차이를 state에 반영하고, 리소스 타입이 지정한 방향으로 명시적으로 복원합니다.
`direction`은 계산 속성입니다. 직접 값을 넣는 대신 ingress 또는 egress 리소스를 선택하세요.

생성 시 빈 설명/생략한 설명은 API null로 반환되며 Terraform 빈 문자열로 표현합니다.
빈 값과 null로 설명을 수정하면 기존 값이 유지되므로 기존 설명을 비우려면 삭제 후 다시 생성해야 합니다.
`security_group_id` 변경도 교체입니다. 기존 설명이 비어 있지 않은데 새 값이 unknown이면 교체를 보수적으로 계획합니다.
교체 중 필터링 공백이 생길 수 있습니다. 적용 전에 검토하고 공백을 허용할 수 없으면 `prevent_destroy`를 사용하세요.
`create_before_destroy`가 성공한다고 가정하지 마세요. API가 완전히 같은 규칙의 중복 생성을 거부하며 Provider는 이 오류 후 기존 규칙을 채택하지 않습니다.
다른 규칙을 생성하는 것은 실패한 요청의 자동 재시도가 아닙니다.

규칙의 title/content는 원문으로 반환됐습니다. URL 인코딩하거나 HTML 디코딩하지 않습니다.
그룹 설명과는 계약이 다릅니다. IPv4 CIDR의 호스트 비트도 그대로 보존하며 패킷 동작을 추측하지 않습니다.
단일 포트와 양 끝이 같은 범위는 동일한 `from_port`/`to_port` 값으로 표현합니다.
TCP/UDP, 포트 1–65535, IPv4 CIDR만 받습니다. 기록한 실험에서 포트 0, bare IP 수정, IPv6/ICMP 입력은 실패했습니다.
이 결과는 실제 패킷 적용이나 모든 계정/존 변형을 검증한 것은 아닙니다.

## 조회와 부재 판단

API는 개별 상세 대신 부모별 규칙 목록을 제공합니다.
Provider는 페이지 없는 전체 배열·count·정확한 정수 ID·필드를 검증한 뒤 ID를 찾습니다.
실제 규칙 53개 실험에서 기본 조회와 `page_no`/`page_size=1` 조회 모두 53개를 반환했습니다. 페이지 인수는 무시됐습니다.
예상 밖 페이지 메타데이터, 잘못된 행, 중복 ID나 API 오류가 있으면 state를 지우지 않고 조회를 중단합니다.
이는 50개 경계를 넘은 근거이며 모든 미래 계정 quota가 무제한이라는 뜻은 아닙니다.

먼저 부모 상세를 읽습니다. 검증된 부모 HTTP 200/빈 배열/count 0 계약으로 부모 부재를 판단합니다.
부모가 존재하면 성공한 전체 규칙 목록에 정확한 ID가 없을 때만 자식 부재로 판단합니다.
부모 삭제 후 규칙 endpoint는 `CHECK_PARAM`을 반환할 수 있으며, 이 오류만으로 부재로 처리하지 않습니다.
부모가 사라지면 자식을 더 이상 지정할 수 없지만, 물리적인 연쇄 삭제나 과금 종료를 독립적으로 입증하지는 못합니다.

## 실패 복구

생성 시 반환된 정수 ID를 정확한 십진수 문자열로 변환하고, 나머지 응답/Read 검증 오류를 보고하기 전에 복합 ID를 state에 저장합니다.
Terraform이 객체를 tainted로 표시할 수 있으므로 다음 apply 전에 실제 객체와 교체 계획을 확인하세요.
ID가 불명확하면 다른 객체를 생성하기 전에 비공개 요청 기록과 목록을 대조하세요.
쓰기는 한 번만 시도합니다. API 오류 때문에 POST/PUT/DELETE를 무작정 재전송하거나 동명 객체를 채택하지 않습니다.

수정 실패는 이전 state를 유지하며, 서비스에 반영됐을 수 있는 결과는 refresh로 확인합니다.
삭제는 먼저 조회하고 DELETE를 최대 한 번 전송한 뒤 정확한 규칙 부재를 확인해야 성공합니다.
삭제/후속 조회 실패 시 state를 유지합니다. 규칙 리소스가 부모나 다른 규칙을 삭제하지 않습니다.
비밀번호, 토큰, 원본 응답 본문은 공개 리소스 state나 진단에 복사하지 않습니다.

## 근거와 실행

T058은 두 방향 리소스, ID 유지 수정, 전체 import 비교, 두 방향의 영속 재import 후 무변경 plan,
방향 drift 복원, 설명 초기화/부모 변경의 명시적인 교체 계획, 외부 삭제/재생성, 의존성 순서 destroy를 다룹니다.
합성 테스트는 생성 실패 ID 정리, 수정/삭제 실패 state, 부모 부재, 잘못된 입력, timeout만 변경과 unknown 설명의 계획을 추가로 검증합니다.
실환경 테스트는 이번 실행의 새 부모·자식 ID만 Git 밖에 기록하고, 부모를 삭제하기 전에 자식 부재를 확인합니다.

```sh
TF_ACC=1 IWINV_LIVE_TERRAFORM_RULE_WRITE=1 \
  IWINV_TEST_JOURNAL_DIR=/absolute/private/mode0700/directory \
  go test -race ./internal/provider -run '^TestAccSecurity(Rules|EgressRule)$' -v -timeout 20m
```

[개발 가이드](../../../design/ko/development.md)의 환경변수 인증정보를 사용하세요. 이 opt-in은 CI에서 켜지 않습니다.
state, 원본 출력과 정리 대장은 비공개로 보관하세요. 실패하거나 ID가 불명확한 생성은 확인·복구 대상이며 요청 반복의 근거가 아닙니다.

공식 출처: [목록](https://iwinv.readme.io/reference/get_v1-security-groups-id-rules), [생성](https://iwinv.readme.io/reference/post_v1-security-groups-id-rules), [수정](https://iwinv.readme.io/reference/put_v1-security-groups-id-rules-rule-id), [삭제](https://iwinv.readme.io/reference/delete_v1-security-groups-id-rules-rule-id).
