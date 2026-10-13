package help

import (
 "errors"
 "net/url"
 "os"
 "path/filepath"
 "strings"
)

// ConfiguredService always exposes the public manual. An unavailable optional
// provider is reported to its caller without blocking manual annotation.
func ConfiguredService(get func(string)string)(*Service,error){
 d,e:=EmbeddedDocument();if e!=nil{return nil,e}
 service:=NewService(d,nil)
 endpoint,keyFile:=get("APP_HELP_API_URL"),get("APP_HELP_API_KEY_FILE")
 if endpoint==""&&keyFile==""{return service,nil}
 if !filepath.IsAbs(keyFile){return service,errors.New("help key must be an external absolute file")}
 u,e:=url.Parse(endpoint);if e!=nil||u.Scheme!="https"||u.Host==""||u.User!=nil||u.RawQuery!=""||u.Fragment!=""{return service,errors.New("help API requires a configured HTTPS endpoint")}
 info,e:=os.Stat(keyFile);if e!=nil||!info.Mode().IsRegular()||info.Size()>4096{return service,errors.New("help key unavailable")}
 raw,e:=os.ReadFile(keyFile);if e!=nil{return service,errors.New("help key unavailable")}
 key:=strings.TrimSpace(string(raw));if key==""||strings.ContainsAny(key,"\r\n\x00"){return service,errors.New("help key invalid")}
 model:=get("APP_HELP_MODEL");if model==""{model="glm-4.7-flash"}
 if len(model)>128||strings.ContainsAny(model,"\r\n\x00"){return service,errors.New("help model invalid")}
 service.Provider=HTTPProvider{Endpoint:endpoint,Key:key,Model:model};return service,nil
}
