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

const (
	Pending = "-1"
	Created = "1"
	Local   = "2"
)

func main() {
	//Teste()
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
	Comandosql string `json:"comandosql"`
	Situacao   string `json:"situacao"`
}

type Comandos struct {
	Comandos []Comando `xml:"comando"`
}

type Comando struct {
	SQL string `xml:",cdata"`
}

// dispatch  -
func scriptDetail(c *gin.Context) {
	scriptTime := c.Query("scripttime")
	if scriptTime == "" {
		c.String(http.StatusBadRequest, "'scriptTime' não enviado")
		return
	}

	// ler arquivo
	// verificar se tem que ser o mesmo em toda a aplicação guid

	if scriptTime != "" {
		scripts := []Script{}
		crudResponseDb := GetScriptsDB(scriptTime, scripts, c)
		crudResponseLocaly := GetScriptsLocaly(scriptTime, crudResponseDb, c)

		c.JSON(http.StatusOK, crudResponseLocaly)
	} else {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: uuid.New().String(), Header: "parou tudo", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Warrrrning"})
		c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": -1, "mensagem": "cliente não exportou nenhuma regra"})
	}

}

func GetScriptsDB(scriptTime string, scripts []Script, c *gin.Context) (allScripts []Script) {
	cmds, err := crudadapter.EvalQuery("monarca/getSituacao", "", config.PreSharedToken, crudadapter.M{"arquivo": scriptTime}, "")
	if err := cmds[0].RowsAsStructs(&scripts); err != nil {
		log.Println("erro para fazer o parse ", err.Error())
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if err != nil {
		log.Println("Erro ao executar a query", err.Error())
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	return scripts
}

func GetScriptsLocaly(scriptTime string, localyScripts []Script, c *gin.Context) (allScripts []Script) {

	if len(localyScripts) == 0 {
		// caminho da pasta no config
		xmlFile := readFile(config.FilePath+scriptTime+config.Extension, c)

		var comandos Comandos

		if err := xml.Unmarshal(xmlFile, &comandos); err != nil {
			SimLogger.Local.Send(SimLogger.ShortLog{Session: uuid.New().String(), Header: "parou tudo", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Warrrrning"})
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": -1, "mensagem": "cliente não exportou nenhuma regra"})
			return
		}

		for _, comando := range comandos.Comandos {

			re := regexp.MustCompile(`\s+`)
			mapperReponse := Script{
				Comandosql: re.ReplaceAllString(comando.SQL, " "),
				Situacao:   Local,
			}
			c.JSON(http.StatusOK, mapperReponse)
		}

	} else {
		c.JSON(http.StatusOK, localyScripts)
	}

	return localyScripts
}

func readFile(filePath string, c *gin.Context) []byte {
	xml, err := os.ReadFile(filePath)
	if err != nil {
		SimLogger.Local.Send(SimLogger.ShortLog{Session: uuid.New().String(), Header: "parou tudo", Level: SimLogger.LOG_LEVEL_ERROR, Body: "Warrrrning"})
		c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": -1, "mensagem": "cliente não exportou nenhuma regra"})
		return xml
	}
	return xml
}
