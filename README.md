# repo-judger" 
## 기존 judger에서 변경된 점
- main.go에 rabbitMQ와 연결하는 부분이 추가되었습니다.
- rabbitMQ에서 읽어온 메시지를 기반으로 소스코드를 저장하는 부분이 추가되었습니다. 저장되는 소스코드는 JSON 코드에 첨부된 "language" 부분에 따라 형식을 다르게 저장할 수 있습니다.

## 추가로 구현해야 하는 부분
- rabbitMQ에서 읽어온 코드를 실행한 결과를 MySQL에 저장합니다. 이 때, JSON값으로 주어진 request_id 값을 identifier로 하여 사용자가 자신의 코드 결과를 찾을 수 있도록 설계합니다.
