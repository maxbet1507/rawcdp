# rawcdp
chrome remote debugging protocol for golang

## インストール

```
go get github.com/maxbet1507/rawcdp
```

## 説明

[Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/)の低レイヤー実装です。
ほとんどWebSocketをWrappingしている程度なので、__EXPERIMENTAL__になっているものも実行することができます。

以下はPage.navigateしてページの全画面キャプチャーする処理のサンプルです。

```golang
func screenshot(ctx context.Context, url string) ([]byte, error) {
	client, err := rawcdp.Connect("http://localhost:9222/json", nil)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	batch := rawcdp.Batch{}

	batch.Call("Page.enable", nil, nil)
	batch.Call("Page.navigate", map[string]interface{}{
		"url": url,
	}, nil)
	batch.Listen("Page.loadEventFired", nil)

	doc := struct {
		Root struct {
			NodeID int64 `json:"nodeId"`
		} `json:"root"`
	}{}
	batch.Call("DOM.getDocument", nil, &doc)

	sel := struct {
		NodeID int64 `json:"nodeId"`
	}{}
	batch.Call("DOM.querySelector", map[string]interface{}{
		"selector": "body",
		"nodeId":   &doc.Root.NodeID,
	}, &sel)

	box := struct {
		Model struct {
			Width  int64 `json:"width"`
			Height int64 `json:"height"`
		} `json:"model"`
	}{}
	batch.Call("DOM.getBoxModel", map[string]interface{}{
		"nodeId": &sel.NodeID,
	}, &box)
	batch.Call("Emulation.setDeviceMetricsOverride", map[string]interface{}{
		"width":             &box.Model.Width,
		"height":            &box.Model.Height,
		"deviceScaleFactor": 0,
		"mobile":            false,
	}, nil)

	cap := struct {
		Data []byte `json:"data"`
	}{}
	batch.Call("Page.captureScreenshot", nil, &cap)

	if err := batch.Run(ctx, client); err != nil {
		return nil, err
	}
	return cap.Data, nil
}
```

便利なstructなどは提供していないので、自前で用意する必要はあります。

rawcdp.ClientがWebSocketの直接のWrapperですが、
そのまま使用するのはつらいのでrawcdp.Batchを経由して使うことを推奨します。
