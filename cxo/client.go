package cxo

import (
	"errors"
	"fmt"
	"github.com/evanlinjin/bbs/typ"
	"github.com/skycoin/cxo/node"
	"github.com/skycoin/cxo/skyobject"
	"github.com/skycoin/skycoin/src/cipher"
	"github.com/skycoin/skycoin/src/cipher/encoder"
	"strconv"
	"time"
)

// CXOConfig represents a configuration for CXO Client.
type CXOConfig struct {
	Master bool
	Port   int
}

// NewCXOConfig creates a new CXOConfig.
func NewCXOConfig() *CXOConfig {
	c := CXOConfig{
		Master: false,
		Port:   8998,
	}
	return &c
}

// Client acts as a client for cxo.
type Client struct {
	config   *CXOConfig
	cxo      *node.Client
	uManager *typ.UserManager
	bManager *typ.BoardManager
}

// NewClient creates a new Client.
func NewClient(conf *CXOConfig) (*Client, error) {

	// Setup cxo client.
	clientConfig := node.NewClientConfig()
	client, e := node.NewClient(clientConfig)
	if e != nil {
		return nil, e
	}
	c := Client{
		config:   conf,
		cxo:      client,
		uManager: typ.NewUserManager(),
		bManager: typ.NewBoardManager(conf.Master),
	}
	c.initUsers()
	c.initBoards()
	return &c, nil
}

func (c *Client) initUsers() {
	m := c.uManager
	if m.Load() != nil {
		m.Clear()
		m.AddNewRandomMaster()
		if e := m.Save(); e != nil {
			panic(e)
		}
	}
}

func (c *Client) initBoards() {
	m := c.bManager
	if m.Load() != nil {
		m.Clear()
		if e := m.Save(); e != nil {
			panic(e)
		}
	}
}

// Launch runs the Client.
func (c *Client) Launch() error {
	// Connect to cxo daemon.
	if e := c.cxo.Start("[::]:" + strconv.Itoa(c.config.Port)); e != nil {
		return e
	}
	// Wait for safety.
	time.Sleep(5 * time.Second)
	// Register schemas for cxo.
	e := c.cxo.Execute(func(ct *node.Container) error {
		ct.Register("Board", typ.Board{})
		ct.Register("Thread", typ.Thread{})
		ct.Register("Post", typ.Post{})
		ct.Register("ThreadPage", typ.ThreadPage{})
		ct.Register("BoardContainer", typ.BoardContainer{})
		return nil
	})
	if e != nil {
		return e
	}
	// Load boards from BoardManager.
	for _, bc := range c.bManager.Boards {

		// [USE THIS PART WHEN KONSTANTIN GIVES ME FIX]
		//_, e := c.Subscribe(bc.PubKey)
		//if e != nil {
		//	fmt.Println("[BOARD CONFIG]", bc.PubKey.Hex(), e)
		//	fmt.Println("[BOARD CONFIG] Removing", bc.PubKey.Hex())
		//	c.bManager.RemoveConfig(bc.PubKey)
		//}

		// [TEMPORARY WORKAROUND]
		if bc.Master {
			if c.config.Master == false {
				continue
			}
			c.bManager.RemoveConfig(bc.PubKey)
			_, _, e := c.InjectBoard(bc)
			if e != nil {
				fmt.Println("[BOARD CONFIG]", bc.PubKey.Hex(), e)
				fmt.Println("[BOARD CONFIG] Removing", bc.PubKey.Hex())
				c.bManager.RemoveConfig(bc.PubKey)
			}
		} else {
			_, e := c.Subscribe(bc.PubKey)
			if e != nil {
				fmt.Println("[BOARD CONFIG]", bc.PubKey.Hex(), e)
				fmt.Println("[BOARD CONFIG] Removing", bc.PubKey.Hex())
				c.bManager.RemoveConfig(bc.PubKey)
			}
		}
	}
	return e
}

// Shutdown shutdowns the Client.
func (c *Client) Shutdown() error {
	return c.cxo.Close()
}

// Subscribe subscribes to a board.
func (c *Client) Subscribe(pk cipher.PubKey) (*typ.BoardConfig, error) {
	// Check if we already have the config, if not make one.
	bc, e := c.bManager.GetConfig(pk)
	if e != nil {
		bc, _ = typ.NewBoardConfig(pk, "")
	}
	// Attempt subscribe in cxo.
	if c.cxo.Subscribe(pk) == false {
		return nil, errors.New("subscription failed")
	}
	// Add to BoardManager.
	c.bManager.AddConfig(bc)
	return bc, nil
}

// Unsubscribe unsubscribes from a board.
func (c *Client) Unsubscribe(pk cipher.PubKey) bool {
	c.bManager.RemoveConfig(pk)
	return c.cxo.Unsubscribe(pk)
}

// InjectBoard injects a new master board with given BoardConfig.
func (c *Client) InjectBoard(bc *typ.BoardConfig) (
	b *typ.Board, bCont *typ.BoardContainer, e error,
) {
	if c.config.Master == false {
		e = errors.New("action not allowed; node is not master")
		return
	}
	// Make Board from BoardConfig.
	b = typ.NewBoardFromConfig(bc)
	// Check if BoardConfig exists.
	if c.bManager.HasConfig(bc.PubKey) == true {
		e = errors.New("board already exists")
		return
	}
	// Add to cxo.
	e = c.cxo.Execute(func(ct *node.Container) error {
		r := ct.NewRoot(bc.PubKey, bc.SecKey)
		boardRef := ct.Save(*b)
		bCont = typ.NewBoardContainer(boardRef)
		r.Inject(*bCont, bc.SecKey)
		return nil
	})
	if e != nil {
		return
	}
	// Subscribe to board.
	if c.cxo.Subscribe(bc.PubKey) == false {
		e = errors.New("failed to subscribe to board in cxo")
		return
	}
	// Add config to BoardManager.
	e = c.bManager.AddConfig(bc)
	return
}

// ObtainBoard obtains a board of specified PubKey from cxo.
// Also obtains the BoardContainer.
// Assumes public key provided is valid.
func (c *Client) ObtainBoard(bpk cipher.PubKey) (
	b *typ.Board, bCont *typ.BoardContainer, e error,
) {
	bCont = new(typ.BoardContainer)
	b = new(typ.Board)

	e = c.cxo.Execute(func(ct *node.Container) error {
		r := ct.Root(bpk)
		if r == nil {
			c.bManager.RemoveConfig(bpk)
			return errors.New("nil root, removed from config")
		}
		values, e := r.Values()
		if e != nil {
			return e
		}
		//if len(values) != 1 {
		//	return errors.New("invalid root")
		//}
		// Loop through and get latest BoardContainer.
		for _, v := range values {
			bContTemp := &typ.BoardContainer{}
			// Get BoardContainer.
			if e := encoder.DeserializeRaw(v.Data(), bContTemp); e != nil {
				return e
			}
			if bContTemp.Seq >= bCont.Seq {
				bCont = bContTemp
			}
		}
		// Get Board.
		bValue, e := ct.GetObject("Board", bCont.Board)
		if e != nil {
			return e
		}
		if e := encoder.DeserializeRaw(bValue.Data(), b); e != nil {
			return e
		}
		b.PubKey = bpk.Hex()
		return nil
	})
	return
}

// ObtainAllBoards obtains all the boards that we are subscribed to.
func (c *Client) ObtainAllBoards() (
	bList []*typ.Board, bContList []*typ.BoardContainer, e error,
) {
	// Get board configs.
	bConfList := c.bManager.GetList()
	// Prepare outputs.
	bList = make([]*typ.Board, len(bConfList))
	bContList = make([]*typ.BoardContainer, len(bConfList))
	// Loop.
	for i := 0; i < len(bConfList); i++ {
		b, bCont, err := c.ObtainBoard(bConfList[i].PubKey)
		if err != nil {
			e = err
			return
		}
		bList[i], bContList[i] = b, bCont
	}
	return
}

// InjectThread injects a thread in specified board.
func (c *Client) InjectThread(bpk cipher.PubKey, thread *typ.Thread) (
	b *typ.Board, bCont *typ.BoardContainer, tList []*typ.Thread, e error,
) {
	if c.config.Master == false {
		e = errors.New("action not allowed; node is not master")
		return
	}
	// Check thread.
	if e = thread.CheckAndPrep(); e != nil {
		return
	}
	// Obtain board configuration.
	bc, e := c.bManager.GetConfig(bpk)
	if e != nil {
		return
	}
	// See if we are master of the board.
	if bc.Master == false {
		e = errors.New("not master")
		return
	}
	// Obtain board and board container.
	b, bCont, e = c.ObtainBoard(bpk)
	if e != nil {
		return
	}
	e = c.cxo.Execute(func(ct *node.Container) error {
		var (
			e     error
			tp    *typ.ThreadPage
			tpRef skyobject.Reference
		)
		// Obtain root.
		r := ct.Root(bpk)
		if r == nil {
			return errors.New("nil root")
		}
		// Save thread in container.
		tRef := ct.Save(*thread)
		// Add thread to BoardContainer if not exist.
		if e = bCont.AddThread(tRef); e != nil {
			goto InjectThreadListThreads
		}
		// Add empty ThreadPage to BoardContainer.
		tp = typ.NewThreadPage(tRef)
		tpRef = ct.Save(*tp)
		bCont.AddThreadPage(tpRef)
		// Increment sequence of BoardContainer.
		bCont.Touch()
		// Inject in root.
		r.Inject(*bCont, bc.SecKey)

	InjectThreadListThreads:
		// Obtain all threads in board.
		tList = make([]*typ.Thread, len(bCont.Threads))
		for i, tRef := range bCont.Threads {
			tList[i] = typ.InitThread(tRef)
			v, e2 := ct.GetObject("Thread", tRef)
			if e2 != nil {
				return e2
			}
			if e2 := encoder.DeserializeRaw(v.Data(), tList[i]); e2 != nil {
				return e2
			}
		}
		return e
	})
	return
}

// ObtainThread obtains a single thread.
func (c *Client) ObtainThread(bpk cipher.PubKey, tRef skyobject.Reference) (
	b *typ.Board, bCont *typ.BoardContainer, t *typ.Thread, e error,
) {
	// Obtain board and board container.
	b, bCont, e = c.ObtainBoard(bpk)
	if e != nil {
		return
	}
	e = c.cxo.Execute(func(ct *node.Container) error {
		// Find thread.
		t := &typ.Thread{}
		tVal, e2 := ct.GetObject("Thread", tRef)
		if e2 != nil {
			return e2
		}
		if e2 = encoder.DeserializeRaw(tVal.Data(), t); e2 != nil {
			return e2
		}
		return nil
	})
	return
}

// ObtainThreads obtains threads of a specified board.
func (c *Client) ObtainThreads(bpk cipher.PubKey) (
	b *typ.Board, bCont *typ.BoardContainer, tList []*typ.Thread, e error,
) {
	// Obtain board and board container.
	b, bCont, e = c.ObtainBoard(bpk)
	if e != nil {
		return
	}
	e = c.cxo.Execute(func(ct *node.Container) error {
		// Obtain all threads in board.
		tList = make([]*typ.Thread, len(bCont.Threads))
		for i, tRef := range bCont.Threads {
			tList[i] = typ.InitThread(tRef)
			v, e := ct.GetObject("Thread", tRef)
			if e != nil {
				return e
			}
			if e := encoder.DeserializeRaw(v.Data(), tList[i]); e != nil {
				return e
			}
		}
		return nil
	})
	return
}

// ObtainPosts obtains the posts of specified board and thread.
func (c *Client) ObtainPosts(bpk cipher.PubKey, tRef skyobject.Reference) (
	b *typ.Board, bCont *typ.BoardContainer, t *typ.Thread, tPage *typ.ThreadPage, pList []*typ.Post,
	tpRef skyobject.Reference, e error,
) {
	// Obtain board, board container and thread.
	b, bCont, t, e = c.ObtainThread(bpk, tRef)
	if e != nil {
		return
	}
	e = c.cxo.Execute(func(ct *node.Container) error {
		// Find thread page.
		tpFound := false
		tPage = &typ.ThreadPage{}
		for _, tpRef = range bCont.ThreadPages {
			tpVal, e2 := ct.GetObject("ThreadPage", tpRef)
			if e2 != nil {
				return e2
			}
			if e2 := encoder.DeserializeRaw(tpVal.Data(), tPage); e2 != nil {
				return e2
			}
			if tPage.Thread == tRef {
				tpFound = true
				break
			}
		}
		if tpFound == false {
			return errors.New("thread page not found")
		}
		// Obtain posts from thread page.
		pList = make([]*typ.Post, len(tPage.Posts))
		for i, pRef := range tPage.Posts {
			pVal, e2 := ct.GetObject("Post", pRef)
			if e2 != nil {
				return e2
			}
			pList[i] = &typ.Post{}
			if e2 := encoder.DeserializeRaw(pVal.Data(), pList[i]); e2 != nil {
				return e2
			}
			pList[i].CheckCreator()
		}
		return nil
	})
	return
}

// InjectPost injects a post in specified Board and Thread.
// Note that post needs to be signed properly.
func (c *Client) InjectPost(bpk cipher.PubKey, tRef skyobject.Reference, post *typ.Post) (
	b *typ.Board, bCont *typ.BoardContainer,
	t *typ.Thread, tPage *typ.ThreadPage, pList []*typ.Post, e error,
) {
	// Obtain posts and whatnot.
	oldTpRef := skyobject.Reference{}
	b, bCont, t, tPage, pList, oldTpRef, e = c.ObtainPosts(bpk, tRef)
	if e != nil {
		return
	}
	// Obtain board configuration.
	bc, e := c.bManager.GetConfig(bpk)
	if e != nil {
		return
	}
	// See if we are master of the board.
	if bc.Master == false {
		e = errors.New("not master")
		return
	}
	// Check post to inject.
	if e = post.CheckContent(); e != nil {
		return
	}
	if e = post.CheckCreator(); e != nil {
		return
	}
	if e = post.CheckSig(); e != nil {
		return
	}
	// Touch post.
	post.Touch()

	e = c.cxo.Execute(func(ct *node.Container) error {
		// Save post in cxo container and obtain it's reference.
		pRef := ct.Save(*post)

		// Add post to thread page and save new thread page in cxo container.
		// Hence, obtain the new thread page's reference.
		tPage.AddPost(pRef)
		newTpRef := ct.Save(tPage)

		// Replace thread page reference in board container.
		// Hence, increment sequence of ThreadContainer.
		if e2 := bCont.ReplaceThreadPage(oldTpRef, newTpRef); e2 != nil {
			return e2
		}
		bCont.Touch()

		// Inject new board container in root.
		r := ct.Root(bpk)
		r.Inject(bCont, bc.SecKey)

		// Add post to output post list.
		pList = append(pList, post)
		return nil
	})
	return
}
