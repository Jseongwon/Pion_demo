// -----------------------------------------------------------------------------
// pion_demo.go — STUN / TURN / ICE playground in one self‑contained Go file.
// -----------------------------------------------------------------------------
//
// ✦ 목적 --------------------------------------------------------------------
//
//	• STUN 서버 실습   : RFC 5389 Binding Request ↔ Binding Success Response.
//	• TURN 서버 실습   : RFC 5766 UDP 릴레이 + Long‑Term Credentials 인증.
//	• ICE 연결 실험    : 두 프로세스를 실행하여 후보 수집 → 연결 체크.
//
//	학습자가 '네트워크 패킷 흐름'과 'Pion API 사용법'을 동시에 익힐 수 있게
//	모든 코드를 한 파일로 단순화했습니다. 필요한 옵션은 플래그로 노출하여
//	"go run" 만으로 실행 가능하도록 구성했습니다.
//
// ✦ 사용 방법 ----------------------------------------------------------------
//
//	# STUN 서버 (UDP 3478)
//	go run pion_demo.go -mode stun -listen :3478
//
//	# TURN 서버 (UDP 3478) — realm "example.org", 사용자 demo/demo
//	go run pion_demo.go -mode turn -listen :3478 -relay 0.0.0.0:0 \
//	     -realm example.org -user demo -pass demo
//
//	# ICE 데모 — 하나는 controlling, 하나는 controlled 로 실행
//	# 1️⃣ 터미널 A (controlling)
//	go run pion_demo.go -mode ice -controlling -stun 127.0.0.1:3478
//	# 2️⃣ 터미널 B (default controlled)
//	go run pion_demo.go -mode ice
//	#   A가 출력한 base64 문자열을 B에 붙여넣고 두 프로세스를 연결
//
// -----------------------------------------------------------------------------
package main

// ───────────────────────────── Imports ───────────────────────────────────────
// 표준 라이브러리 ------------------------------------------------------------
import (
	"bufio"           // stdin 입력을 읽어 상대 ICE credentials 전달
	"context"         // 컨텍스트 사용
	"encoding/base64" // ICE credentials를 편히 복사/붙여넣기 위해 base64 인코딩
	"flag"            // 명령행 옵션 파싱
	"fmt"             // 콘솔 출력
	"log"             // 간단한 로깅 헬퍼
	"net"             // 네트워크 I/O (UDP/TCP)
	"os"              // stdin 사용
	"strings"         // 문자열 유틸리티

	// 타임아웃
	// 외부 라이브러리 ---------------------------------------------------------
	ice "github.com/pion/ice/v2" // ICE 에이전트
	"github.com/pion/stun"       // STUN 메시지 파싱/빌드
	"github.com/pion/turn/v2"    // TURN 서버 구현체
)

// ───────────────────────────── main() ───────────────────────────────────────
func main() {
	// ▸ 공통 플래그 -----------------------------------------------------------
	mode := flag.String("mode", "stun", "stun | turn | ice 중 선택")
	listen := flag.String("listen", ":3478", "STUN/TURN 수신 주소 (udp/tcp)")

	// TURN 모드 플래그 --------------------------------------------------------
	relay := flag.String("relay", "0.0.0.0:0", "TURN 릴레이용 로컬 바인드")
	realm := flag.String("realm", "example.org", "TURN Realm (Long‑Term Auth)")
	user := flag.String("user", "demo", "TURN username")
	pass := flag.String("pass", "demo", "TURN password")

	// ICE 모드 플래그 ---------------------------------------------------------
	controlling := flag.Bool("controlling", false, "true → ICE‑controlling")
	stunSrv := flag.String("stun", "", "ICE 후보 수집용 STUN 호스트:포트")
	turnSrv := flag.String("turn", "", "ICE 후보 수집용 TURN 호스트:포트")
	pskey := flag.String("secret", "", "피어에게 받은 Base64 ICE creds")

	flag.Parse()

	// 모드별 진입점 -----------------------------------------------------------
	switch *mode {
	case "stun":
		if err := runStunServer(*listen); err != nil {
			log.Fatal(err)
		}
	case "turn":
		if err := runTurnServer(*listen, *relay, *realm, *user, *pass); err != nil {
			log.Fatal(err)
		}
	case "ice":
		if err := runIceDemo(*controlling, *stunSrv, *turnSrv, *pskey); err != nil {
			log.Fatal(err)
		}
	default:
		flag.Usage() // 잘못된 mode 입력 시 사용법 출력
	}
}

// -----------------------------------------------------------------------------
// 1. STUN 서버 구현  (UDP)
// -----------------------------------------------------------------------------
// ▼ runStunServer는 가장 단순한 Binding‑only STUN 서버 예시입니다.
//
//	▸ 클라이언트 패킷 수신 → stun.Message 파싱 → Binding Success 응답 전송.
//	▸ XOR-Mapped‑Address 속성에 클라이언트(원본) IP/Port 를 넣어 돌려줍니다.
//	▸ RFC 8489 이후 MESSAGE-INTEGRITY, FINGERPRINT 권장 하지만 데모이므로 단순화.
func runStunServer(addr string) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	log.Printf("[STUN] listening on %s", conn.LocalAddr())

	buf := make([]byte, 1500)
	for {
		n, raddr, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		msg := new(stun.Message)
		msg.Raw = append([]byte{}, buf[:n]...)
		if err := msg.Decode(); err != nil {
			log.Printf("[STUN] non-STUN packet from %s: %v", raddr, err)
			continue
		}
		if msg.Type.Method != stun.MethodBinding || msg.Type.Class != stun.ClassRequest {
			continue
		}
		resp := stun.MustBuild(
			msg,
			stun.BindingSuccess,
			&stun.XORMappedAddress{
				IP:   raddr.(*net.UDPAddr).IP,
				Port: raddr.(*net.UDPAddr).Port,
			},
			stun.Fingerprint,
		)
		if _, err := conn.WriteTo(resp.Raw, raddr); err != nil {
			log.Printf("[STUN] send error: %v", err)
		}
	}
}

// parseIPPort : net.Addr → (IP, Port) --------------------------------------
func parseIPPort(addr net.Addr) (net.IP, int) {
	if u, ok := addr.(*net.UDPAddr); ok {
		return u.IP, u.Port
	}
	return net.IPv4zero, 0
}

// -----------------------------------------------------------------------------
// 2. TURN 서버 구현 (UDP 릴레이)
// -----------------------------------------------------------------------------
func runTurnServer(listen, relay, realm, username, password string) error {
	log.Printf("[TURN] server starting on %s (realm=%s)", listen, realm)

	packetConn, err := net.ListenPacket("udp4", listen)
	if err != nil {
		return err
	}

	s, err := turn.NewServer(turn.ServerConfig{
		Realm: realm,
		AuthHandler: func(usernameInput, realmInput string, srcAddr net.Addr) ([]byte, bool) {
			if usernameInput == username {
				return turn.GenerateAuthKey(username, realm, password), true
			}
			return nil, false
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: packetConn,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(strings.Split(relay, ":")[0]),
					Address:      relay,
				},
			},
		},
	})
	if err != nil {
		return err
	}
	defer s.Close()

	select {}
}

// -----------------------------------------------------------------------------
// 3. ICE 데모 (p2p candidate 수집 & 연결)
// -----------------------------------------------------------------------------
func runIceDemo(controlling bool, stunSrv, turnSrv, secret string) error {
	var urls []*ice.URL
	if stunSrv != "" {
		u, err := ice.ParseURL("stun://" + stunSrv)
		if err != nil {
			return err
		}
		urls = append(urls, u)
	}
	if turnSrv != "" {
		u, err := ice.ParseURL("turn://" + turnSrv + "?transport=udp")
		if err != nil {
			return err
		}
		urls = append(urls, u)
	}

	config := &ice.AgentConfig{
		Urls:         urls,
		NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
	}

	agent, err := ice.NewAgent(config)
	if err != nil {
		return err
	}
	defer agent.Close()

	// Credentials
	ufrag, pwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		return err
	}

	if controlling {
		fmt.Println("=== 아래 문자열을 상대에게 전달하세요 (base64) ===")
		offer := fmt.Sprintf("%s:%s", ufrag, pwd)
		fmt.Println(base64.StdEncoding.EncodeToString([]byte(offer)))
	}

	// 상대방 credentials 입력
	var remoteUfrag, remotePwd string
	if secret == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("피어 base64 creds: ")
		line, _ := reader.ReadString('\n')
		secret = strings.TrimSpace(line)
	}
	dec, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return err
	}
	parts := strings.SplitN(string(dec), ":", 2)
	remoteUfrag, remotePwd = parts[0], parts[1]
	if err = agent.SetRemoteCredentials(remoteUfrag, remotePwd); err != nil {
		return err
	}

	// 상태 변화 콜백
	agent.OnConnectionStateChange(func(state ice.ConnectionState) {
		log.Printf("[ICE] state → %s", state)
	})

	// 후보 수집 시작
	if err = agent.GatherCandidates(); err != nil {
		return err
	}

	// 연결 시도 (컨트롤러만)
	if controlling {
		conn, err := agent.Dial(context.Background(), remoteUfrag, remotePwd)
		if err != nil {
			return err
		}
		defer conn.Close()
		fmt.Println("ICE 연결 성공!")
	} else {
		conn, err := agent.Accept(context.Background(), ufrag, pwd)
		if err != nil {
			return err
		}
		defer conn.Close()
		fmt.Println("ICE 연결 성공!")
	}

	select {}
}

// sendPing : 연결된 뒤 선택된 CandidatePair 정보 출력 ------------------------
func sendPing(a *ice.Agent) {
	// 선택된 후보 쌍 정보를 가져오기 위해 OnSelectedCandidatePairChange 이벤트 사용
	a.OnSelectedCandidatePairChange(func(local, remote ice.Candidate) {
		log.Printf("local  → %s (%s)", local, local.Type())
		log.Printf("remote → %s (%s)", remote, remote.Type())
	})
}
