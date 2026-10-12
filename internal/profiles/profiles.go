package profiles

import (
 "crypto/rand"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "sort"
 "strings"
 "github.com/gofrs/flock"
 "golang.org/x/crypto/bcrypt"
)

type Input struct { Name, Library, Username, Password string; Port int; LAN bool }
type Profile struct {
 ID string `json:"id"`
 Name string `json:"name"`
 Library string `json:"library"`
 Username string `json:"username"`
 PasswordHash string `json:"passwordHash"`
 DataDir string `json:"-"`
 Port int `json:"port"`
 LAN bool `json:"lan"`
}
func Create(root string,i Input)(Profile,error){
 if strings.TrimSpace(i.Name)==""||len(i.Name)>80||strings.TrimSpace(i.Username)==""||len(i.Password)<10||len(i.Password)>72||i.Port<1024||i.Port>65535 {return Profile{},errors.New("provide a profile name, username, 10–72 byte password, and port 1024–65535")}
 root,err:=filepath.Abs(root);if err!=nil{return Profile{},err};if err=os.MkdirAll(root,0700);err!=nil{return Profile{},err}
 lock:=flock.New(filepath.Join(root,".profiles.lock"));if err=lock.Lock();err!=nil{return Profile{},err};defer lock.Unlock()
 existing,err:=List(root);if err!=nil{return Profile{},err};for _,p:=range existing{if p.Port==i.Port{return Profile{},errors.New("port already assigned to another profile")}}
 path,err:=filepath.Abs(i.Library);if err!=nil{return Profile{},err};path,err=filepath.EvalSymlinks(path);if err!=nil{return Profile{},fmt.Errorf("music folder unavailable: %w",err)};info,err:=os.Stat(path);if err!=nil||!info.IsDir(){return Profile{},errors.New("choose an existing music folder")}
 probe,err:=os.CreateTemp(path,".tuneharbor-access-");if err!=nil{return Profile{},errors.New("music folder must be writable")};probe.Close();os.Remove(probe.Name())
 hash,err:=bcrypt.GenerateFromPassword([]byte(i.Password),bcrypt.DefaultCost);if err!=nil{return Profile{},err};id:=make([]byte,16);if _,err=rand.Read(id);err!=nil{return Profile{},err}
 p:=Profile{ID:hex.EncodeToString(id),Name:strings.TrimSpace(i.Name),Library:path,Username:strings.TrimSpace(i.Username),PasswordHash:string(hash),Port:i.Port,LAN:i.LAN};p.DataDir=filepath.Join(root,p.ID)
 if err=os.Mkdir(p.DataDir,0700);err!=nil{return Profile{},err};ok:=false;defer func(){if !ok{os.RemoveAll(p.DataDir)}}()
 data,err:=json.MarshalIndent(p,"","  ");if err!=nil{return Profile{},err};err=os.WriteFile(filepath.Join(p.DataDir,"profile.json"),data,0600);if err!=nil{return Profile{},err};ok=true;return p,nil
}
func Open(root,id string)(Profile,error){
 if len(id)!=32{return Profile{},errors.New("invalid profile ID")};if _,err:=hex.DecodeString(id);err!=nil{return Profile{},errors.New("invalid profile ID")}
 dir:=filepath.Join(root,id);info,err:=os.Lstat(dir);if err!=nil{return Profile{},err};if !info.IsDir()||info.Mode()&os.ModeSymlink!=0{return Profile{},errors.New("invalid profile directory")}
 data,err:=os.ReadFile(filepath.Join(dir,"profile.json"));if err!=nil{return Profile{},err};var p Profile;if err=json.Unmarshal(data,&p);err!=nil{return Profile{},err};if p.ID!=id{return Profile{},errors.New("profile identity mismatch")};p.DataDir=dir;return p,nil
}
func List(root string)([]Profile,error){
 dirs,err:=os.ReadDir(root);if os.IsNotExist(err){return []Profile{},nil};if err!=nil{return nil,err};out:=[]Profile{};for _,d:=range dirs{if !d.IsDir(){continue};if len(d.Name())!=32{continue};p,e:=Open(root,d.Name());if e!=nil{return nil,fmt.Errorf("profile %s: %w",d.Name(),e)};out=append(out,p)};sort.Slice(out,func(i,j int)bool{return out[i].Name<out[j].Name});return out,nil
}
