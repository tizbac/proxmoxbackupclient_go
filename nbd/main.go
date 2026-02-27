package main

import (
	"fmt"
	"pbscommon"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	client := &pbscommon.PBSClient{}
	app := tview.NewApplication()
	loading_modal := tview.NewModal().SetText("Connecting to server...")
	error_modal := tview.NewModal()
	txt_pbs_server := tview.NewInputField().SetLabel("PBS Server").SetPlaceholder("https://1.2.3.4:8007").SetFieldWidth(30)
	txt_api_token := tview.NewInputField().SetLabel("API Token").SetPlaceholder("root@pam!yourtoken").SetFieldWidth(30)
	txt_secret := tview.NewInputField().SetLabel("PBS Secret").SetPlaceholder("a-b-c-d").SetFieldWidth(30)
	dataset_namespace := tview.NewInputField().SetLabel("Dataset / Namespace").SetPlaceholder("dataset/namespace1/namespace2").SetFieldWidth(30)
	
	snaproot := tview.NewTreeNode("/").SetColor(tcell.ColorDarkRed)
	snaplist := tview.NewTreeView().SetRoot(snaproot)
	form := tview.NewForm().AddFormItem(txt_pbs_server).
	AddFormItem(txt_api_token).
	AddFormItem(txt_secret).
	AddFormItem(dataset_namespace).
	AddButton("Next", func() {
		app.SetRoot(loading_modal, false)
		ns := strings.Split(dataset_namespace.GetText(), "/")
		client = &pbscommon.PBSClient{
		BaseURL:         txt_pbs_server.GetText(),
		CertFingerPrint: "", //"ea:7d:06:f9:87:73:a4:72:d0:e8:05:a4:b3:3d:95:d7:0a:26:dd:6d:5c:ca:e6:99:83:e4:11:3b:5f:10:f4:4b",
		AuthID:          txt_api_token.GetText(),
		Secret:          txt_secret.GetText(),
		Datastore:       ns[0],
		Namespace:       strings.Join(ns[1:],"/"),
		Insecure:        true,
		}

		snap, err := client.ListSnapshots()
		if err != nil {
			error_modal.SetText(err.Error())
			app.SetRoot(error_modal, true)
		}else{	
			snaproot.ClearChildren()
			for _ , sn := range snap {
				/*snaplist.AddItem(sn.BackupID+" "+time.Unix(sn.BackupTime,0).Format("2006-01-02 15:04:05"), sn.BackupType, '', func ()  {
					
				})*/
				node := tview.NewTreeNode(sn.BackupType+" "+sn.BackupID+" "+time.Unix(sn.BackupTime,0).Format("2006-01-02 15:04:05"))
				for _, x := range sn.Files {
					node2 := tview.NewTreeNode(x.Filename)
					if strings.HasSuffix(x.Filename, ".fidx") {
						node2.SetSelectable(true)
						node2.SetColor(tcell.ColorGreen)
						node2.SetSelectedFunc(func() {
							client.Manifest = sn
							app.SetRoot(loading_modal, false)
							client.Connect(true)
							data, err := client.DownloadToBytes(x.Filename)
							if err != nil {
								error_modal.SetText(err.Error()+fmt.Sprintf("%+v \n%+v", x, sn))
								app.SetRoot(error_modal, true)
							}else{
								app.Stop()
								fmt.Println(len(data))
							}
						})
						
					}else{
						node2.SetSelectable(false)
					}
					

					node.AddChild(node2)
				}
				node.SetSelectable(false)
				snaproot.AddChild(node)
			}
			app.SetRoot(snaplist, true)
		}
	}).
	AddButton("Cancel", func() {
		app.Stop()
	})
	form.SetBorder(true).SetTitle("PBS Connection details").SetTitleAlign(tview.AlignLeft)
	if err := app.SetRoot(form, true).EnableMouse(true).EnablePaste(true).Run(); err != nil {
		panic(err)
	}
	/*

	l, err := net.Listen("unix", "/tmp/pbsnbd")
	if err != nil {
		panic(err)
	}
*/


}
