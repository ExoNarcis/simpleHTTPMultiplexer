# Pet2
### Simple HTTP Multiplexer   
### приложение представляет собой http-сервер с одним хендлером   
### хендлер на вход получает POST-запрос со списком url в json-формате   
### сервер запрашивает данные по всем этим url и возвращает результат клиенту в json-формате   
### если в процессе обработки хотя бы одного из url получена ошибка, обработка всего списка прекращается и клиенту возвращается текстовая ошибка    
### сервер не принимает запрос если количество url в в нем больше чем указано в методе Init (maxurls)
### сервер не обслуживает больше чем указанное в Init одновременных входящих http-запросов (maxPostCount)
### таймаут на обработку одного входящего запроса указывается в вызове NewhttpMultiplexer (serverTimeout)
### для каждого входящего запроса есть ограничение на n исходящих указанно в Init (maxGetCount)
### таймаут на запрос одного url указывается в NewhttpMultiplexer (clientTimeout)
### обработка запроса может быть отменена клиентом в любой момент, это должно повлечь за собой остановку всех операций связанных с этим запросом   
### сервис поддерживает 'graceful shutdown': при получении сигнала от OS перестает принимать входящие запросы, завершает текущие запросы и остановиться усли в Init указан true (saveshutdown)  

#### NewhttpMultiplexer(port string, serverTimeout uint, clientTimeout uint)
#### (... *httpMultiplexer)Init(maxPostCount uint, maxGetCount uint, maxurls uint, saveshutdown bool)

### Пример:
```
package main
import (
	"github.com/ExoNarcis/Pet2/httpMultiplexer"
)

func main() {
	Serv := httpMultiplexer.NewhttpMultiplexer("3333", 10, 1)
	Serv.Init(100, 4, 20, true)
}
```
