package client

// ForOperation scopes replay advice to the selected catalog operation. A header
// alone does not establish server deduplication; writes still never auto-retry.
func (c *Client) ForOperation(keyed bool) *Client {
	clone := *c
	clone.keyedOperation = keyed
	return &clone
}
