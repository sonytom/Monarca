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

type Script struct {
	Comandosql string   `json:"comandosql"`
	Situacao   Elements `json:"situacao"`
}

type ScriptResponse struct {
	Comandosql string `json:"comandosql"`
	Situacao   string `json:"situacao"`
}

type Elements struct {
	Elements []int `json:"Elements"`
}

type Comandos struct {
	Comandos []Comando `xml:"comando"`
}

type Comando struct {
	SQL string `xml:",cdata"`
}

func scriptDetail(c *gin.Context) {
	scriptTime := c.Query("scripttime")
	if scriptTime == "" {
		c.String(http.StatusBadRequest, "'scriptTime' não enviado")
		return
	}

	// verificar se tem que ser o mesmo em toda a aplicação guid

	if scriptTime != "" {

		reponseDB, err := GetScriptsDB(scriptTime)

		if err != nil {
			log.Println("Erro ao executar a query", err.Error())
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": -1, "mensagem": "cliente não exportou nenhuma regra"})
			return
		}

		mapperResponse := make([]ScriptResponse, 0)

		mapperResponse = setMessages(reponseDB, mapperResponse)

		if len(reponseDB) == 0 {
			responseLocaly, err := GetScriptsLocaly(scriptTime, reponseDB)

			mapperResponse = setMessages(responseLocaly, mapperResponse)
			if err != nil {
				log.Println("Erro ao executar a query", err.Error())
				c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": -1, "mensagem": "cliente não exportou nenhuma regra"})
				return
			}
		}
		c.JSON(http.StatusOK, mapperResponse)
	} else {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: uuid.New().String(), Header: "parou tudo", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Warrrrning"})
		c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": -1, "mensagem": "cliente não exportou nenhuma regra"})
	}
}

func setMessages(scripts []Script, mapperResponse []ScriptResponse) []ScriptResponse {
	for i, comando := range scripts {
		mapperResponse = append(mapperResponse, ScriptResponse{Comandosql: comando.Comandosql, Situacao: "Pendente"})
		if scripts[i].Situacao.Elements[len(scripts[i].Situacao.Elements)-1] == 1 {
			mapperResponse[i].Situacao = "Sucesso"
		}

		if scripts[i].Situacao.Elements[len(scripts[i].Situacao.Elements)-1] == -2 {
			mapperResponse[i].Situacao = "Não Executado"
		}
	}
	return mapperResponse
}

func GetScriptsDB(scriptTime string) ([]Script, error) {
	scripts := make([]Script, 0)
	cmds, err := crudadapter.EvalQuery("monarca/getSituacao", "", config.PreSharedToken, crudadapter.M{"arquivo": scriptTime}, "")
	if err := cmds[0].RowsAsStructs(&scripts); err != nil {
		log.Println("erro para fazer o parse ", err.Error())
		return scripts, err
	}
	if err != nil {
		log.Println("Erro ao executar a query", err.Error())
		return scripts, err
	}
	return scripts, nil
}

func GetScriptsLocaly(scriptTime string, localyScripts []Script) ([]Script, error) {
	xmlFile, err := readFile(config.FilePath + scriptTime + config.Extension)

	if err != nil {
		log.Println("Erro ao executar a query", err.Error())
		return localyScripts, err
	}
	var comandos Comandos
	if err := xml.Unmarshal(xmlFile, &comandos); err != nil {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: uuid.New().String(), Header: "parou tudo", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Warrrrning"})
		return localyScripts, err
	}

	novo := make([]Script, len(comandos.Comandos))

	for i, tt := range comandos.Comandos {
		re := regexp.MustCompile(`\s+`)
		novo[i].Comandosql = re.ReplaceAllString(tt.SQL, " ")
		novo[i].Situacao.Elements = []int{-2}
	}

	return novo, nil
}

func readFile(filePath string) ([]byte, error) {
	xml, err := os.ReadFile(filePath)
	if err != nil {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: uuid.New().String(), Header: "parou tudo", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Warrrrning"})
		return xml, err
	}
	return xml, err
}
