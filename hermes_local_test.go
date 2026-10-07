package main

import (
 "os"
 "testing"
)

// Run explicitly with HERMES_FEISHU_ENV_PATH set. Never print credentials or chat IDs.
func TestLocalHermesFeishuConfigReadOnly(t *testing.T){
 path:=os.Getenv("HERMES_FEISHU_ENV_PATH")
 if path=="" {t.Skip("set HERMES_FEISHU_ENV_PATH to verify the local Hermes runtime")}
 c,err:=loadHermesFeishu(path)
 if err!=nil{t.Fatal(err)}
 if c.AppID=="" || c.AppSecret=="" || c.ChatID=="" || c.FeishuBase=="" {t.Fatal("Hermes Feishu config incomplete")}
}
