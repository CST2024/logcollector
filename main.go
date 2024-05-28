package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type LogEntry struct {
	from   string
	to     string
	reject bool
}

// コネクションを保持するための配列
var connections = make([]net.Conn, 128)

// 拒否するホストの配列
var rejects = make([]string, 0)

// ログのバッファ
var logs = make([]LogEntry, 4096)

var logFile *os.File

func main() {
	// .envを読み込む
	godotenv.Load(".env")

	PORT := ":" + os.Getenv("LC_PORT")
	PROTOCOL := "tcp"

	if PORT == ":" {
		PrintError("ポートが指定されなかったため、8122を使用します")
		PORT = ":8122"
	}

	f, err := os.Open("rejects.txt")
	if err == nil {
		buffer := make([]byte, 2048)
		len, err := f.Read(buffer)
		if err == nil {
			data := string(buffer[:len])
			rejects = append(rejects, strings.Split(data, "\n")...)
		}
	}
	f.Close()

	logFile, _ = os.Create("./log.txt")
	defer logFile.Close()

	// 標準入力の処理
	go func() {
		var mode, data string
		fmt.Scanf("%s %s", &mode, &data)
		if mode == "ADD" {
			addReject(data)
		}
		if mode == "REMOVE" {
			removeReject(data)
		}
	}()

	tcpAddr, err := net.ResolveTCPAddr(PROTOCOL, PORT)
	if err != nil {
		PrintError(err.Error())
		return
	}

	listener, err := net.ListenTCP(PROTOCOL, tcpAddr)
	if err != nil {
		PrintError(err.Error())
		return
	}

	PrintInfo(fmt.Sprintf("Listening at localhost%s...", PORT))
	for {
		con, err := listener.Accept()
		if err != nil {
			PrintError(err.Error())
			continue
		} else {
			PrintInfo(fmt.Sprintf("New conenction from %s", con.RemoteAddr().String()))
		}

		// 新しい接続を非同期で処理する
		go handleConnect(con)
	}
}

func handleConnect(conn net.Conn) {
	// 終了時の処理を定義
	defer func() {
		// コネクションの一覧から削除
		newConnections := connections[0:0]
		for _, v := range connections {
			if v != nil && v.LocalAddr() != conn.LocalAddr() {
				newConnections = append(newConnections, v)
			}
		}
		connections = newConnections

		conn.Close()
	}()

	buff := make([]byte, 1024)

	// rejectsを送信する
	for _, v := range rejects {
		conn.Write([]byte(fmt.Sprintf("ADD\t%s", v)))
		conn.Read(buff)
	}

	// このコネクションを追加する
	connections = append(connections, conn)

	for {
		// 送られてきたメッセージを取得
		messageLength, err := conn.Read(buff)
		if err != nil {
			break
		}
		// バッファからデータを取り出す
		message := string(buff[:messageLength])

		// 送られてきたデータを分割
		data := strings.Split(message, "\t")
		// 配列の長さが３の場合のみ正しい情報として扱う
		if len(data) == 3 {
			var log LogEntry
			log.from = data[0]
			log.to = data[1]
			log.reject, _ = strconv.ParseBool(data[2])
			logs = append(logs, log)

			if log.reject {
				logFile.Write([]byte(fmt.Sprintf("[%d]REJECT FROM:%s\tTO:%s\n", time.Now().Unix(), log.from, log.to)))
				PrintWarn(fmt.Sprintf("%s -> %s | %s", data[0], data[1], data[2]))
			} else {
				logFile.Write([]byte(fmt.Sprintf("[%d]PERMIT FROM:%s\tTO:%s\n", time.Now().Unix(), log.from, log.to)))
				PrintInfo(fmt.Sprintf("%s -> %s | %s", data[0], data[1], data[2]))
			}
		}

		// 返す
		conn.SetWriteDeadline(time.Now().Add(time.Minute))
		conn.Write([]byte("Accepted"))
	}

}

func PrintInfo(message string) {
	fmt.Printf("\033[38;5;83m[%s] [Inf] %s\033[0m\n", time.Now().Format("2006-01-02 15:04:05"), message)
}

func PrintWarn(message string) {
	fmt.Printf("\033[38;5;130m[%s] [Wrn] %s\033[0m\n", time.Now().Format("2006-01-02 15:04:05"), message)
}

func PrintError(message string) {
	fmt.Printf("\033[38;5;124m[%s] [Err] %s\033[0m\n", time.Now().Format("2006-01-02 15:04:05"), message)
}

func addReject(host string) {
	rejects = append(rejects, host)
	go sendAll(fmt.Sprintf("ADD %s", host))
}

func removeReject(host string) {
	var newRejects []string
	for _, v := range rejects {
		if v != host {
			newRejects = append(newRejects, v)
		}
	}
	rejects = newRejects
}

// 全てのコネクションにメッセージを送信する
func sendAll(message string) {
	for _, v := range connections {
		v.Write([]byte(message))
	}
}
