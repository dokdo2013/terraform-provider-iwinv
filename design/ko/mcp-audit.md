# MCP 기능 조사와 OAuth 검증

[English](../en/mcp-audit.md) · [기능 범위](coverage.md) · 실측: 2026-09-19

## 근거와 현재 미완료 조건

C27과 T014는 미완료입니다. [기계 판독 기록](../inventory/mcp.json)은 공개 발견·등록과 인증된 도구 조사를 구분합니다.
아직 인증된 목록을 얻지 못했으므로 `tools`·`tool_count`는 null입니다. 서버에 도구가 없다는 뜻이 아닙니다.
Terraform Provider는 직접 서비스 API를 호출하며 MCP 실행 의존성을 추가하지 않습니다.

공식 연결 주소는 `https://mcp.iwinv.kr`입니다. 토큰 없는 initialize 요청은 HTTP 401, `mcp:tools` scope challenge와
보호 리소스 메타데이터 URL을 반환했습니다. 발견된 인증 서버는 `https://oauth.iwinv.kr`이며 authorization-code와
S256 PKCE를 지원합니다. 실제 challenge는 광고된 전체 scope보다 좁아 `mcp:tools`만 요청했고 profile·offline access는
요청하지 않았습니다. HMAC control-plane 키를 OAuth에 보내거나 bearer token으로 사용하지 않았습니다.

임시 환경의 공식 MCP Python SDK 2.2.0으로 동적 등록했으며 HTTP 201과 등록 관리 URL·토큰을 받았습니다. authorization-code
grant만 요청하고 loopback callback과 state/issuer 검증을 준비했으며, 등록 근거는 Git 밖의 0600 파일로 보관했습니다.
기존 Codex/MCP 설정은 변경하지 않았습니다.

브라우저는 로그인과 새 클라이언트의 계정 접근 권한 부여를 함께 수행하는 iwinv 화면에 도달했습니다. 소유자 동의는 하지
않았습니다. 승인 대기 중 SDK 세션이 종료됐으며 access token, 인증된 initialize/tools/list 결과는 받지 않았습니다. SDK의
구체적인 중첩 실패 원인은 보존되지 않아 이를 iwinv 토큰 endpoint 결함의 근거로 삼지 않습니다. 로컬 listener도 종료됐습니다.

**새로 등록했지만 권한을 부여하지 않은 클라이언트 한 개**의 정리는 미확인입니다. 반환된 관리 URL과 관리 토큰으로 DELETE를
요청했으나 HTTP 404였고, 같은 URL의 후속 GET은 HTTP 200과 바로 그 신규 client ID를 반환했습니다. 관리 토큰·URL 회전도
없었습니다. 삭제 성공이나 등록 비활성화를 주장할 수 없습니다. 다른 경로·메서드를 추측하거나 추가 등록하지 않았습니다.
권한 부여나 새 클라이언트 생성 전에 등록 관리·정리 제약을 해결해야 합니다. 이 조사에서 MCP tools/call·클라우드 리소스 변경·
메일 발송·DNS 변경은 없었습니다.

등록 ID·관리 토큰·브라우저 인증 URL·원응답은 비공개 로컬 근거에만 보관하며 공개 저장소에 넣지 않습니다. 관측 시간 초과만으로
클라이언트를 다시 만들지 마세요. 수명주기와 인증 조건을 해결한 뒤 기록된 클라이언트를 대조하거나 재사용해야 합니다.
별도 웹메일 테스트 서비스는 이후 콘솔 삭제 이력과 빈 서비스 목록으로 정리를 확인했습니다.
이 사실은 MCP 임시 등록의 정리 실패나 전체 T056의 합격을 의미하지 않습니다.

## 오프라인 캡처 검증

T076은 자격증명 없는 준비 도구의 검증이며 **인증된 MCP acceptance가 아닙니다**. `scripts/summarize_mcp.py`는 순서대로 기록한
비공개 SDK 페이지를 읽습니다. 각 페이지에는 실제 요청에 사용한 cursor와 SDK 응답이 있어야 합니다.

```json
{
  "request_cursor": null,
  "response": {
    "tools": [{"name": "example_read", "inputSchema": {"type": "object", "properties": {}}}],
    "nextCursor": null
  }
}
```

의도적으로 캡처한 파일만 요청 순서로 지정합니다.

```sh
python3 scripts/summarize_mcp.py /absolute/private/page-0.json /absolute/private/page-1.json
python3 -m unittest discover -s scripts -p 'test_summarize_mcp.py' -v
```

누락·순서 변경·반복·미완료 페이지, 중복되거나 잘못된 도구 이름, 잘못된 입력 형태와 boolean 힌트를 거절합니다. 이름,
최상위 입력 이름·타입·필수 여부, 명시된 boolean annotation, 출력 스키마 존재 여부만 내보냅니다. 설명·기본값·예시·중첩 스키마 내용·
`_meta`·연속 cursor는 출력하지 않고 오류에서도 캡처 값을 제외합니다. 선언 타입의 빈 목록은 직접 `type` 선언이 없다는 뜻이며,
입력이 아무 값도 받지 않는다는 뜻이 아닙니다. boolean 속성 스키마는 true/false를 명시적으로 보존합니다. 조합 스키마·제약·중첩 스키마는 비공개 원본으로 추가 검토해야 합니다.
이 요약은 전체 JSON Schema나 개별 작업의 API 계약이 아닙니다.

도구/속성 이름 자체에도 비공개 정보가 있을 수 있으므로 공개 전에 직접 검토해야 합니다. 이름이나 annotation을 근거로 안전성·
멱등성·실행 권한을 추정하지 마세요. 스크립트는 네트워크 요청·도구 호출을 하지 않고 inventory 파일도 자동으로 쓰지 않습니다.
합성 페이지·정보 제외·실패 검증은 실환경 키 없는 문서 CI에서 실행합니다.

같은 날 후속 대조에서도 반환된 정확한 관리 URI가 HTTP 200과 같은 등록을 반환했습니다. 새 클라이언트·동의·토큰은 만들지 않았습니다. 비공개 기술지원 초안에 등록 정리 문제를 포함했으며 아직 제출하지 않았습니다. [실행 기록](contract-progress.md)을 참고하세요.

## 남은 작업

1. 임시 클라이언트 등록 정리를 해결하고 검증합니다.
2. 구체적 scope에 대한 소유자 OAuth 동의를 받은 뒤 기록된 지원 흐름으로 인증합니다.
3. 요청 cursor를 포함한 전체 도구 목록을 얻어 작업·입력을 API/CLI 대장과 대조합니다.
4. 전체 입출력 스키마는 비공개로 검토하고 계정과 무관한 계약만 직접 검토해 공개합니다.
5. 권한·세션·등록 정리도 검증합니다. 도구 목록만으로 CRUD나 Terraform 지원을 주장하지 않습니다.

출처: [iwinv MCP 가이드](https://docs.iwinv.kr/developers/mcp/),
[iwinv 연결 절차](https://docs.iwinv.kr/developers/mcp/codex),
[리소스 메타데이터](https://mcp.iwinv.kr/.well-known/oauth-protected-resource),
[인증 메타데이터](https://oauth.iwinv.kr/.well-known/oauth-authorization-server),
[MCP 인증 명세](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization),
[공식 Python SDK OAuth 가이드](https://py.sdk.modelcontextprotocol.io/client/oauth-clients/).
