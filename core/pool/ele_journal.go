package pool

import (
	"errors"
	"io"
	"os"
	"reflect"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils/log"
)

// errNoActiveJournal is returned if a element is attempted to be inserted
// into the journal, but no such file is currently open.
var errNoActiveJournal = errors.New("no active journal")

// devNull is a WriteCloser that just discards anything written into it. Its
// goal is to allow the element journal to write into a fake journal when
// loading elements on startup without printing warnings due to no file
// being readt for write.
type devNull struct{}

func (*devNull) Write(p []byte) (n int, err error) { return len(p), nil }
func (*devNull) Close() error                      { return nil }

// eleJournal is a rotating log of elements with the aim of storing locally
// created elements to allow non-executed ones to survive node restarts.
type eleJournal struct {
	path     string         // Filesystem path to store the elements at
	writer   io.WriteCloser // Output stream to write new elements into
	elemType reflect.Type   // The true type of the element such as Transaction or TxPackage
}

// newEleJournal creates a new element journal to
func newEleJournal(path string, elemType reflect.Type) *eleJournal {
	return &eleJournal{
		path:     path,
		elemType: elemType,
	}
}

// load parses a element journal dump from disk, loading its contents into
// the specified pool.
func (journal *eleJournal) load(add func([]Element) []error) error {
	// Skip the parsing if the journal file doens't exist at all
	if _, err := os.Stat(journal.path); os.IsNotExist(err) {
		return nil
	}
	// Open the journal for loading any past elements
	input, err := os.Open(journal.path)
	if err != nil {
		return err
	}
	defer input.Close()

	// Temporarily discard any journal additions (don't double add on load)
	journal.writer = new(devNull)
	defer func() { journal.writer = nil }()

	// Inject all elements from the journal into the pool
	stream := rlp.NewStream(input, 0)
	total, dropped := 0, 0

	// Create a method to load a limited batch of elements and bump the
	// appropriate progress counters. Then use this method to load all the
	// journalled elements in small-ish batches.
	loadBatch := func(elems []Element) {
		for _, err := range add(elems) {
			if err != nil {
				log.Debug("Failed to add journaled element", "err", err)
				dropped++
			}
		}
	}
	var (
		failure error
		batch   []Element
	)
	for {
		// Parse the next element and terminate on error
		elePtr := reflect.New(journal.elemType)
		if err = stream.Decode(elePtr.Interface()); err != nil {
			if err != io.EOF {
				failure = err
			}
			if len(batch) > 0 {
				loadBatch(batch)
			}
			break
		}
		// New element parsed, queue up for later, import if threnshold is reached
		total++

		if batch = append(batch, elePtr.Interface().(Element)); len(batch) > 1024 {
			loadBatch(batch)
			batch = batch[:0]
		}
	}
	log.Info("Loaded local element journal", "elements", total, "dropped", dropped, "elementType", journal.elemType.String())

	return failure
}

// insert adds the specified element to the local disk journal.
func (journal *eleJournal) insert(ele Element) error {
	if journal.writer == nil {
		return errNoActiveJournal
	}
	if err := rlp.Encode(journal.writer, ele); err != nil {
		return err
	}
	return nil
}

// rotate regenerates the element journal based on the current contents of
// the element pool.
func (journal *eleJournal) rotate(all map[common.Address][]Element) error {
	// Close the current journal (if any is open)
	if journal.writer != nil {
		if err := journal.writer.Close(); err != nil {
			return err
		}
		journal.writer = nil
	}
	// Generate a new journal with the contents of the current pool
	replacement, err := os.OpenFile(journal.path+".new", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	journaled := 0
	for _, elems := range all {
		for _, ele := range elems {
			if err = rlp.Encode(replacement, ele); err != nil {
				replacement.Close()
				return err
			}
		}
		journaled += len(elems)
	}
	replacement.Close()

	// Replace the live journal with the newly generated one
	if err = os.Rename(journal.path+".new", journal.path); err != nil {
		return err
	}
	sink, err := os.OpenFile(journal.path, os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		return err
	}
	journal.writer = sink
	log.Info("Regenerated local element journal", "elements", journaled, "accounts", len(all))

	return nil
}

// close flushes the element journal contents to disk and closes the file.
func (journal *eleJournal) close() error {
	var err error

	if journal.writer != nil {
		err = journal.writer.Close()
		journal.writer = nil
	}
	return err
}
