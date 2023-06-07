package main

import (
	"encoding/xml"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	crudadapter "git.simkorp.com.br/repositorio/simkorp/CrudAdapter"
	envcon "git.simkorp.com.br/repositorio/simkorp/Envcon"
	SimLogger "git.simkorp.com.br/repositorio/simkorp/SimLogger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	os.Setenv("SIMLOGGER_COMPONENT", "Monarca")
	os.Setenv("SIMLOGGER_SYSTEM", "simfrete")
	os.Setenv("SIMLOGGER_AMQ_URL", "mqtest.simfrete.com:61616")
	SimLogger.Init()
}

func main() {
	envcon.Load(&config)
	crudadapter.Init(crudadapter.Config{
		CrudUrl: config.Url,
		Debug:   true,
	})
	log.Println(envcon.Dump(config))
	router := gin.Default()
	router.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Accept", "Authorization", "Content-Type", "Referer", "User-Agent"},
		ExposeHeaders:    []string{"Origin", "Content-Length", "Accept", "Authorization", "Content-Type", "Referer", "User-Agent"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	router.GET("scriptdetail", scriptDetail)
	if err := router.Run(config.MakePort()); err != nil {
		log.Fatal(err)
	}
}

const (
	for_empty_structure = -2
)

type Script struct {
	Comandosql string   `json:"comandosql"`
	Situacao   Elements `json:"situacao"`
}

type ResultScriptResponse struct {
	Comandosql string
	Situacao   string
}

type Elements struct {
	Elements []int `json:"Elements"`
}

type Commands struct {
	Commands []Command `xml:"comando"`
}

type Command struct {
	SQL string `xml:",cdata"`
}

func scriptDetail(c *gin.Context) {
	scriptTime := c.Query("scripttime")
	if scriptTime == "" {
		c.String(http.StatusBadRequest, "'scriptTime' não enviado")
		return
	}
	sessionLogId := uuid.New().String()

	if scriptTime != "" {

		crudAdapterResponse, err := GetScriptsDB(scriptTime, sessionLogId)

		if err != nil {
			SimLogger.Local.Send(SimLogger.ShortLog{Session: sessionLogId, Header: "stoped", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Erro ao executar a query : " + err.Error()})
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": -1, "mensagem": "Erro ao executar a query"})
			return
		}

		scriptResponse := make([]ResultScriptResponse, 0)

		scriptResponse = setMessages(crudAdapterResponse, scriptResponse)

		if len(crudAdapterResponse) == 0 {
			localSearch, err := GetScriptsLocaly(scriptTime, crudAdapterResponse, sessionLogId)
			scriptResponse = setMessages(localSearch, scriptResponse)
			if err != nil {
				log.Println("Erro em executar localmente : ", err.Error())
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": -2, "mensagem": "Erro na busca local do arquivo"})
				return
			}
		}
		c.JSON(http.StatusOK, scriptResponse)
	} else {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: sessionLogId, Header: "stoped", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Warrrrning"})
		c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": -1, "mensagem": "deu falha"})
	}
}

func setMessages(scripts []Script, scriptResponse []ResultScriptResponse) []ResultScriptResponse {
	for i, comando := range scripts {
		scriptResponse = append(scriptResponse, ResultScriptResponse{
			Comandosql: comando.Comandosql,
			Situacao:   "Pendente",
		})

		if scripts[i].Situacao.Elements[len(scripts[i].Situacao.Elements)-1] == 1 {
			scriptResponse[i].Situacao = "Sucesso"
		}

		if scripts[i].Situacao.Elements[len(scripts[i].Situacao.Elements)-1] == for_empty_structure {
			scriptResponse[i].Situacao = "Não Executado"
		}
	}
	return scriptResponse
}

func GetScriptsDB(scriptTime string, sessionLogId string) ([]Script, error) {
	scripts := make([]Script, 0)
	cmds, err := crudadapter.EvalQuery("monarca/getSituacao", "", config.PreSharedToken, crudadapter.M{"arquivo": scriptTime}, "")
	if err := cmds[0].RowsAsStructs(&scripts); err != nil {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: sessionLogId, Header: "stoped", Level: SimLogger.LOG_LEVEL_ERROR, Body: "erro para fazer o parse :" + err.Error()})
		return scripts, err
	}
	if err != nil {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: sessionLogId, Header: "stoped", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Erro ao executar a query" + err.Error()})
		return scripts, err
	}
	return scripts, nil
}

func GetScriptsLocaly(scriptTime string, scripts []Script, sessionLogId string) ([]Script, error) {
	xmlFile, err := readFile(config.FilePath+scriptTime+config.Extension, sessionLogId)

	if err != nil {
		log.Println("Erro ao executar a query", err.Error())
		SimLogger.Local.Send(SimLogger.ShortLog{Session: sessionLogId, Header: "stoped", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Erro ao fazer o parse do arquvo xml : " + err.Error()})
		return scripts, err
	}
	var commands Commands
	if err := xml.Unmarshal(xmlFile, &commands); err != nil {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: sessionLogId, Header: "stoped", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Erro ao fazer o unmarshal do objeto : " + err.Error()})
		return scripts, err
	}

	locallyScripts := serializeSql(commands)

	return locallyScripts, nil
}

func serializeSql(commands Commands) []Script {
	locallyScripts := make([]Script, len(commands.Commands))

	for i, command := range commands.Commands {
		re := regexp.MustCompile(`\s+`)
		locallyScripts[i].Comandosql = re.ReplaceAllString(command.SQL, " ")
		locallyScripts[i].Situacao.Elements = []int{for_empty_structure}
	}
	return locallyScripts
}

func readFile(filePath string, sessionLogId string) ([]byte, error) {
	xml, err := os.ReadFile(filePath)
	if err != nil {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: sessionLogId, Header: "stoped", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Erro ao ler o arquivo" + err.Error()})
		return xml, err
	}
	return xml, err
}
