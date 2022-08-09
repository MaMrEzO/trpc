# TRPC
TRPC(Test RPC) is a simple tool for testing RPC's which aim you get response from a RPC and pass it to another one, with a simple syntax.

TRPC (Test Remote Procedure Call) mostly gRPC.

## Syntax

### File header

Every TRPC file starts with these required lines:
```
test "Test title" 		// This is a brife text about what this file going to test
desc "Test description" // A longest test which describe your test(s)...
trpc "v.1.0.0" 			// Syntax version of the file
timeout 7.0             // Expected timeout for every RPC call
```

### Imports

**Note**: If your service supports reflection(must be implemented) you do not need this section at all.

You can import protofile or protoset but not both of them, in case of protofile you can even define `importpath` for protofiles.

```
importpath "path/to/import/path"
import protofile "path/to/protoFile1"
import protofile "path/to/protoFile2"
```

OR

```
import protoset "path/to/protoSet"
```

### Endpoint

Endpoint is your server specification, endpoint definition will be like this:

`endpoint (ENDPOINT_NAME) [insecure] (IP/Domain) port (Port Number) (prefixPath /path/to/endpoint)`

description:

   * `endpoint`: required, reserved word.

   * ENDPOINT_NAME: required, identifier for your server.

   * `insecure`: optional, reserved word, defines an insecure(Plaintext / HTTP/1.1) endpoint.

   * IP / Domain: internet address of your server. `"127.0.0.1"`, `"localhost"` or `"example.com"`

   * `port`: required, number, followed with a number as port of service, like `443` fot HTTPs

   * `prefixPath`: optional, reserved word, followed with a address starts with `/` for example: `/api` 
	
### Invokes 

With invoke section you can call and test RPC's:

```
invoke Invoke_Identifier endpoint_identifier packge.service RPC_Name headers {
	"header-key1": "header-value1",
	"header-key2": "header-value2"
} data {
  //JSON5 
  field1: "field - 1 - value",
  .
  .
  .
} expects {
	//some possible expects 
	response.code isOk()
	response.message isEmpty()
	response.data.field1 isEqual("Expected value")
	response.data.field2 isNull()
	response.data.files isNotEmpty()
	response.data.files["FileName"].name
}
```
