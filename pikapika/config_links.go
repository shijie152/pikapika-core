package pikapika

import (
	"encoding/json"
	"pikapika/pikapika/database/properties"
)

// ---------- 远程配置 ----------

func configUrl() string {
	u, _ := properties.LoadProperty("configUrl", "https://cdn.comicsparks.work/cfg/pikapika/config.json")
	return u
}

// configLinks 远程推荐链接, 失败时返回空
func configLinks() (string, error) {
	body, err := defaultHttpClientGet(configUrl())
	if err != nil {
		return "{}", nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return "{}", nil
	}
	links, _ := json.Marshal(data["links"])
	if links == nil {
		return "{}", nil
	}
	return string(links), nil
}

// appConfig 版本检查等应用配置, 失败时返回空
func appConfig() (string, error) {
	body, err := defaultHttpClientGet(configUrl())
	if err != nil {
		return "{}", nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return "{}", nil
	}
	app, _ := json.Marshal(data["app"])
	if app == nil {
		return "{}", nil
	}
	return string(app), nil
}

// ---------- Pro 校验 (Dart 层已解锁, 保留空实现保证接口完整) ----------

func proInfoAll() (string, error) {
	return `{"pro_info_af":{"is_pro":false,"expire":0},"pro_info_pat":{"is_pro":false,"pat_id":"","bind_uid":"","request_delete":0,"re_bind":0,"error_type":0,"error_msg":"","access_key":""}}`, nil
}

func reloadPro() error {
	return nil
}

func inputCdKey(cdKey string) error {
	return nil
}

func setPatAccessKey(accessKey string) error {
	return nil
}

func reloadPatAccount() error {
	return nil
}

func bindThisAccount() error {
	return nil
}

func clearPat() error {
	return nil
}
