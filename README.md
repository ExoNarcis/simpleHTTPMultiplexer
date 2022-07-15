# Pet2
### Simple HTTP Multiplexer   
#### приложение представляет собой http-сервер с одним хендлером   
#### хендлер на вход получает POST-запрос со списком url в json-формате   
#### сервер запрашивает данные по всем этим url и возвращает результат клиенту в json-формате   
#### если в процессе обработки хотя бы одного из url получена ошибка, обработка всего списка прекращается и клиенту возвращается текстовая ошибка    
#### сервер не принимает запрос если количество url в в нем больше чем указано в методе Init (maxurls)
#### сервер не обслуживает больше чем указанное в Init одновременных входящих http-запросов (maxPostCount)
#### таймаут на обработку одного входящего запроса указывается в вызове NewhttpMultiplexer (serverTimeout)
#### для каждого входящего запроса есть ограничение на n исходящих указанно в Init (maxGetCount)
#### таймаут на запрос одного url указывается в NewhttpMultiplexer (clientTimeout)
#### обработка запроса может быть отменена клиентом в любой момент, это должно повлечь за собой остановку всех операций связанных с этим запросом   
#### сервис поддерживает 'graceful shutdown': при получении сигнала от OS перестает принимать входящие запросы, завершает текущие запросы и остановиться усли в Init указан true (saveshutdown)  

#### NewhttpMultiplexer(port string, serverTimeout uint, clientTimeout uint)
#### (... *httpMultiplexer)Init(maxPostCount uint, maxGetCount uint, maxurls uint, saveshutdown bool)

### Пример:
```
package main
import (
	"github.com/ExoNarcis/Pet2/httpMultiplexer"
)

func main() {
	Serv := httpMultiplexer.NewhttpMultiplexer("8080", 10, 1)
	Serv.Init(100, 4, 20, true)
}
```

#### Пример POST запроса:
http://localhost:8080/
Body(RAW):
```
{"urls":["https://www.google.com/search?q=1","https://www.google.com/search?q=2","https://www.google.com/search?q=3","https://www.google.com/search?q=4","https://www.google.com/search?q=5","https://www.google.com/search?q=6","https://www.google.com/search?q=7","https://www.google.com/search?q=8","https://www.google.com/search?q=8","https://www.google.com/search?q=9","https://www.google.com/search?q=10","https://www.google.com/search?q=11","https://www.google.com/search?q=12","https://www.google.com/search?q=13"]}
```
#### Ответ будет по схеме:
```
[{"Url":"*","Header":"*","Body":"*"},{"Url":"*","Header":"*","Body":"*"},...]
```
#### DOCKER SETUP (Classic):
```
docker build --tag pet2 ./  
docker run pet2  
```

####  DOCKER-COMPOSE:
```
docker-compose build
docker-compose up 
```

#### Известные проблеммы с DOCKER:
docker0 Bridge не получает IP адреса/нет доступа к Интернету в контейнерах:
[wiki.archlinux.org](https://wiki.archlinux.org/title/Docker_(%D0%A0%D1%83%D1%81%D1%81%D0%BA%D0%B8%D0%B9)#docker0_Bridge_%D0%BD%D0%B5_%D0%BF%D0%BE%D0%BB%D1%83%D1%87%D0%B0%D0%B5%D1%82_IP_%D0%B0%D0%B4%D1%80%D0%B5%D1%81%D0%B0/%D0%BD%D0%B5%D1%82_%D0%B4%D0%BE%D1%81%D1%82%D1%83%D0%BF%D0%B0_%D0%BA_%D0%98%D0%BD%D1%82%D0%B5%D1%80%D0%BD%D0%B5%D1%82%D1%83_%D0%B2_%D0%BA%D0%BE%D0%BD%D1%82%D0%B5%D0%B9%D0%BD%D0%B5%D1%80%D0%B0%D1%85)
или 
docker run --publish 8080:8080 pet2 
##### Порядок выполнения и объяснения подхода:
Список url делится на количество горутин по остаточному принципу
20 запросов, 4 одновременно исходяших. 
Первая горутина получает на вход срез с [0:5] вторая [5:10] и т.д
Если количество запросов не делится на количество одновременно исходящих то производится деление по остаточному принципу:
11 запросов 4 исходящих [0:2], [2:5], [5:8], [8:11] то есть первая порция запросов будет url с индексами 0,2,5,8.
Если на этапе выполнения сообщается об ошибке "get/shema/timeout", завершаются текущие перед попыткой записи в канал.
Горутины завершают свою работу после вывода данных так что при получении сигнала от OS, метод [Shutdown](https://pkg.go.dev/net/http#Server.Shutdown)
гарантирует невозможность получения новых запросов текущие горутины завершают свою работу как и положено.
Разделения по пакетам не предвидится в котексе 1 сервиса и введу малого количества кода, как и в официальных либах от go [пример](https://github.com/golang/go/blob/master/src/net/http/server.go)
