package main

import (
	"encoding/json"
	"fmt"
	"judger/language"
	"judger/sandbox"
	"log"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/yaml.v3"
)

// JSON 데이터 구조를 나타내는 구조체 정의
type Message struct {
	Code       string `json:"code"`
	Language   string `json:"language"`
	Request_id string `json:"request_id"`
}

func main() {
	Username := "rabbitmq%20username%20ss"
	Password := "rabbitmq%20password%20ss"
	Host_IP := "119.69.22.170"
	Port_num := "5672"

	// RabbitMQ 채널 URI 작성 및 연결
	rabbitmqURL := fmt.Sprintf("amqp://%s:%s@%s:%s/",
		Username, Password, Host_IP, Port_num)
	conn, err := amqp.Dial(rabbitmqURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	// RabbitMQ 채널 생성
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	fmt.Println("Successfully connected to RabbitMQ")

	// rabbitMQ에서 JSON형식의 데이터 전송받음
	receive_q, err := ch.QueueDeclare(
		"code_queue",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to declare a receive queue: %v", err)
	}

	rcv_msgs, err := ch.Consume(
		receive_q.Name, // 큐 이름
		"",
		true, // 자동 ACK
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	// go func()로 작성되어 있어 interrupt를 걸거나 에러가 발생하기 전까진 계속 실행됨
	// 필요에 따라 한 번만 실행되도록 설정할 수 있음

	// filepath : rabbitMQ로부터 읽어온 코드가 저장될 폴더 경로
	// judger 프로그램과 같이 ./test 폴더를 이용
	filepath := "./test"
	go func() {
		log.Printf("Ready to Receive message...")
		for msg := range rcv_msgs {
			// rabbitMQ에 있는 메시지의 JSON 부분을 parse하여 코드로 저장하는 함수
			requestID := codeExtract(msg.Body, filepath)
			log.Println(requestID)

			// 읽어온 main파일을 실행시킨 후, result를 저장하는 부분
			//기존 judger의 main.go code

			if language, err := readLanguageFile("./languages/sample.yaml"); err != nil {
				fmt.Println(err)
			} else {
				fmt.Println("language", *language)

				res := sandbox.RunSandbox(sandbox.SandboxConfig{
					Target:    language.Compile,
					MemLimit:  language.CompileLimit.Memory,
					TimeLimit: language.CompileLimit.Time,
					MaxOutput: -1,
					ErrorPath: "./test/error.txt",
				})

				fmt.Println("compile:", res)

				fmt.Println("language", *language)

				res = sandbox.RunSandbox(sandbox.SandboxConfig{
					Target:    language.Execute,
					MemLimit:  268435456,
					TimeLimit: 1000,
					MaxOutput: 1024 * 1024,
					ErrorPath: "./test/error.txt",
					Policy:    &language.Policy,
				})

				fmt.Println("execute:", res)
				// TODO : res값을 request_id값과 함께 DB에 저장
				// 원격 서버에서 DB를 여는 방법을 찾지 못한 상태
			}

			//
		}
	}()

	fmt.Println("Press CTRL+C to exit")
	<-make(chan os.Signal, 1)

}

// JSON으로 parse 후 코드를 path에 저장하는 함수
func codeExtract(json_msg []byte, path string) string {
	var message Message

	fmt.Printf("json_msg : %s\n", json_msg)

	// JSON 문자열을 구조체로 파싱
	err := json.Unmarshal([]byte(json_msg), &message)
	if err != nil {
		log.Fatalf("JSON parsing error : %s", err)
	}

	//extract code
	codeValue := message.Code
	codeLang := message.Language
	Reqid := message.Request_id

	fmt.Printf("Code: %s\n", codeValue)
	fmt.Printf("Language: %s\n", codeLang)

	// Language type에 따라 파일 확장자를 다르게 저장할 수 있다
	// 일단은 python의 경우에만 서술
	express := "txt"
	if codeLang == "python" {
		express = "py"
	}

	// main 파일 저장 (존재하지 않으면 생성, 존재하면 덮어쓰기)
	file, err := os.Create(path + "/main." + express)
	if err != nil {
		log.Fatalf("file create error : %s", err)
	}
	defer file.Close()

	// 파일에 "code" 내용 쓰기
	_, err = file.WriteString(codeValue)
	if err != nil {
		log.Fatalf("file write error: %s", err)
	}
	fmt.Println("Code saved successfully.")

	// req ID 리턴
	return Reqid
}

func readLanguageFile(path string) (*language.Language, error) {
	type Config struct {
		Version  string
		Language language.Language
	}

	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		return nil, err
	}

	return &config.Language, nil
}
