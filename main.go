package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"judger/language"
	"judger/sandbox"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
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

	if err != nil {
		log.Fatalf("fail to create directory: %s", err)
	}

	go func() {
		log.Printf("Ready to Receive message...")
		for msg := range rcv_msgs {

			// filepath : rabbitMQ로부터 읽어온 코드가 저장될 폴더 경로
			// judger 프로그램과 같이 ./test 폴더를 이용
			// test 폴더 : 만들었다가 삭제되는 임시 폴더
			filepath := "./test"
			err = os.MkdirAll(filepath, 0755)

			// rabbitMQ에 있는 메시지의 JSON 부분을 parse하여 코드로 저장하는 함수
			requestID, codeLang := codeExtract(msg.Body, filepath)
			log.Println(requestID)

			// ************************************ //
			// 이후 yaml 파일 추가 시 수정해야 하는 part
			// 읽어온 main파일을 실행시킨 후, result를 저장하는 부분
			//기존 judger의 main.go code
			lang_file := "./languages/sample.yaml"
			switch codeLang {
			case "python":
				//lang_file = "./languages/python.yaml"
			case "cpp":
				lang_file = "./languages/sample.yaml"
			case "c":
				lang_file = "./languages/sample.yaml"
			default:
				fmt.Println("./languages/txt.yaml")
			}
			export_path := "./test/output.txt"

			// codeLang에 맞는 yaml 파일 read
			if language, err := readLanguageFile(lang_file); err != nil {
				fmt.Println(err)
			} else {

				// fmt.Println("language:\n", *language)

				res := sandbox.RunSandbox(sandbox.SandboxConfig{
					Target:    language.Compile,
					MemLimit:  language.CompileLimit.Memory,
					TimeLimit: language.CompileLimit.Time,
					MaxOutput: -1,
					ErrorPath: "./test/error.txt",
				})
				fmt.Println("Code_value_1 : \n", res.Code)
				if res.Code != 0 {
					export_path = "./test/compile_error.txt"
				}

				fmt.Println("compile:\n", res)

				res = sandbox.RunSandbox(sandbox.SandboxConfig{
					Target:    language.Execute,
					MemLimit:  268435456,
					TimeLimit: 1000,
					MaxOutput: 1024 * 1024,
					ErrorPath: "./test/error.txt",
					Policy:    &language.Policy,
				})
				fmt.Println("execute:\n", res)
				fmt.Println("Code_value_2 : \n", res.Code)
				if res.Code != 0 {
					export_path = "./test/compile_error.txt"
				}

				// code를 실행한 결과를 DB에 반영
				// 만약 컴파일 에러 또는 런타임 에러가 발생한 경우 그 에러값이 /test/error.txt에 저장되는데, error case를 구별하는 방법을 찾지 못한 상태
				// TODO : error case 판별법을 알게된 후, error.txt파일의 값을 읽어와서 DB에 requestID와 codeLang string과 함께 올리기
				// 현재는 정상적으로 실행된 코드의 결과값만 반영할 수 있음
				WriteDB(requestID, export_path)

				err := os.RemoveAll(filepath)
				if err != nil {
					log.Fatalf("fail to delete directory: %s", err)
				}

			}
		}
	}()

	fmt.Println("Press CTRL+C to exit")
	<-make(chan os.Signal, 1)

}

// rabbitMQ에서 받아온 메시지를 JSON 형식으로 파싱하고
// 파싱한 내용을 바탕으로 소스코드 파일을 저장하는 함수
// return : request_id, language
func codeExtract(json_msg []byte, path string) (string, string) {
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

	switch codeLang {
	case "python":
		express = "py"
	case "cpp":
		express = "cpp"
	case "c":
		express = "c"
	default:
		express = "txt"
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
	file.Close()

	// req ID 리턴
	return Reqid, codeLang
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

// 원격 MySQL에 접속하여 코드 실행 결과를 데이터로 저장하는 함수
func WriteDB(req_id string, path string) {

	// Read Output.txt
	// error  -> ./test/error.txt
	// exit 0 -> ./test/output.txt
	// 파일 내용 읽기
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf(" We couldn't open file... : %v", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("We couldn't read file... : %v", err)
	}

	// 바이트 슬라이스를 문자열로 변환
	res := string(content)

	// 결과 출력
	fmt.Println("result of code : \n", res)

	// MySQL cc_schema에 연결하는 dsn 작성
	dsn := "root:MYSQL_ROOT_PASSWORD_EXAMPLE@tcp(119.69.22.170:19286)/cc_schema"

	// Connect DB(MySQL)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// DB 유효성 확인
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Successfully connected to MySQL database!")

	// INSERT 쿼리를 실행
	insertQuery := `INSERT INTO submissions_coderesult (request_id, result) VALUES (?, ?)`
	result, err := db.Exec(insertQuery, req_id, res)
	if err != nil {
		log.Fatal(err)
	}

	// Debug용
	lastInsertId, err := result.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s, %s\n", req_id, res)
	fmt.Printf("Inserted record ID: %d\n", lastInsertId)
}
