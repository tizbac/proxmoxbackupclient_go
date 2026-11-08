// pbsnbd is the nbd tool on its own, under the name the bare-metal restore
// ISO calls it by (build-clonezilla-iso.yml, the Clonezilla patches).
// Everywhere else it is salvanza-cli attach-nbd.
package main

import "pbsnbd"

func main() { pbsnbd.Main() }
