package tor

import (
	"context"
	"fmt"
	"testing"
)

func TestConnectToTOR(t *testing.T) {
	torController, err:=NewController("/home/user/Allium/allium-server/tests/torController", 5173)
	if err != nil {
		fmt.Println("Sorry bob")
	}
	ctx := context.Background() 
	err = torController.Start(ctx)
	if err != nil{
		fmt.Println(err)
	}
	torController.Stop()
}