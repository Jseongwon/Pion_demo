# Pion_demo

## 프로젝트 목표
이 프로젝트는 P2P(Peer-to-Peer) 통신을 기반으로 한 영상 통화 및 스트리밍 서비스를 구현하는 것을 목표로 합니다. WebRTC 기술을 활용하여 실시간 양방향 통신을 구현하고, STUN/TURN 서버를 통해 NAT 통과 문제를 해결합니다.

## 테스트 절차

### 1. STUN 서버 테스트
STUN 서버는 클라이언트의 공인 IP와 포트를 확인하는 데 사용됩니다.

```bash
# STUN 서버 실행 (UDP 3478 포트)
go run pion_demo.go -mode stun -listen :3478
```

### 2. TURN 서버 테스트
TURN 서버는 NAT 뒤에 있는 클라이언트들을 위한 릴레이 서버입니다.

```bash
# TURN 서버 실행 (UDP 3478 포트)
go run pion_demo.go -mode turn -listen :3478 -relay 0.0.0.0:0 \
     -realm example.org -user demo -pass demo
```

### 3. ICE 연결 테스트
두 개의 터미널을 사용하여 P2P 연결을 테스트합니다.

터미널 A (controlling):
```bash
go run pion_demo.go -mode ice -controlling -stun 127.0.0.1:3478
```

터미널 B (controlled):
```bash
go run pion_demo.go -mode ice
```

터미널 A에서 출력된 base64 문자열을 터미널 B에 입력하여 연결을 설정합니다.

## 향후 개발 방향

### 1. 기본 기능 구현
- [ ] WebRTC 미디어 스트림 처리 구현
- [ ] 오디오/비디오 코덱 설정 및 최적화
- [ ] 네트워크 상태 모니터링 및 품질 조정
- [ ] 에러 처리 및 재연결 메커니즘 구현

### 2. 고급 기능 구현
- [ ] 다중 참가자 지원 (그룹 통화)
- [ ] 화면 공유 기능
- [ ] 채팅 기능
- [ ] 녹화 기능

### 3. 서버 인프라 구축
- [ ] STUN/TURN 서버 클러스터 구성
- [ ] 로드 밸런싱 구현
- [ ] 모니터링 및 로깅 시스템 구축
- [ ] 보안 강화 (DTLS, SRTP 등)

### 4. 사용자 인터페이스
- [ ] 웹 기반 클라이언트 구현
- [ ] 모바일 앱 개발
- [ ] 사용자 인증 및 권한 관리
- [ ] 사용자 설정 및 프로필 관리

### 5. 성능 최적화
- [ ] 네트워크 대역폭 최적화
- [ ] CPU/메모리 사용량 최적화
- [ ] 지연 시간 최소화
- [ ] 스케일링 테스트 및 최적화

## 기술 스택
- Go (백엔드)
- WebRTC (P2P 통신)
- STUN/TURN (NAT 통과)
- WebSocket (시그널링)
- HTML5/JavaScript (웹 클라이언트)

## 참고 자료
- [WebRTC 공식 문서](https://webrtc.org/)
- [Pion WebRTC 문서](https://pion.ly/docs/)
- [STUN/TURN 프로토콜](https://tools.ietf.org/html/rfc5389)
- [ICE 프로토콜](https://tools.ietf.org/html/rfc8445)
