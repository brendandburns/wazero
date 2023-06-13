package default_http

import (
	"context"
	"encoding/binary"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/wasip1"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// ModuleName is the module name WASI functions are exported into.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md
const ModuleName = "default-outgoing-HTTP"

const i32, i64 = wasm.ValueTypeI32, wasm.ValueTypeI64

var le = binary.LittleEndian

// MustInstantiate calls Instantiate or panics on error.
//
// This is a simpler function for those who know the module ModuleName is not
// already instantiated, and don't need to unload it.
func MustInstantiate(ctx context.Context, r wazero.Runtime) {
	if _, err := Instantiate(ctx, r); err != nil {
		panic(err)
	}
}

// Instantiate instantiates the ModuleName module into the runtime.
//
// # Notes
//
//   - Failure cases are documented on wazero.Runtime InstantiateModule.
//   - Closing the wazero.Runtime has the same effect as closing the result.
func Instantiate(ctx context.Context, r wazero.Runtime) (api.Closer, error) {
	return NewBuilder(r).Instantiate(ctx)
}

// Builder configures the ModuleName module for later use via Compile or Instantiate.
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type Builder interface {
	// Compile compiles the ModuleName module. Call this before Instantiate.
	//
	// Note: This has the same effect as the same function on wazero.HostModuleBuilder.
	Compile(context.Context) (wazero.CompiledModule, error)

	// Instantiate instantiates the ModuleName module and returns a function to close it.
	//
	// Note: This has the same effect as the same function on wazero.HostModuleBuilder.
	Instantiate(context.Context) (api.Closer, error)
}

// NewBuilder returns a new Builder.
func NewBuilder(r wazero.Runtime) Builder {
	return &builder{r}
}

type builder struct{ r wazero.Runtime }

// hostModuleBuilder returns a new wazero.HostModuleBuilder for ModuleName
func (b *builder) hostModuleBuilder() wazero.HostModuleBuilder {
	ret := b.r.NewHostModuleBuilder(ModuleName)
	exportFunctions(ret)
	return ret
}

// Compile implements Builder.Compile
func (b *builder) Compile(ctx context.Context) (wazero.CompiledModule, error) {
	return b.hostModuleBuilder().Compile(ctx)
}

// Instantiate implements Builder.Instantiate
func (b *builder) Instantiate(ctx context.Context) (api.Closer, error) {
	return b.hostModuleBuilder().Instantiate(ctx)
}

// FunctionExporter exports functions into a wazero.HostModuleBuilder.
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type FunctionExporter interface {
	ExportFunctions(wazero.HostModuleBuilder)
}

func NewFunctionExporter() FunctionExporter {
	return &functionExporter{}
}

type functionExporter struct{}

// ExportFunctions implements FunctionExporter.ExportFunctions
func (functionExporter) ExportFunctions(builder wazero.HostModuleBuilder) {
	exportFunctions(builder)
}

func exportFunctions(builder wazero.HostModuleBuilder) {
	exporter := builder.(wasm.HostFuncExporter)

	exporter.ExportHostFunc(request)
	exporter.ExportHostFunc(handle)
}

func newHostFunc(
	name string,
	goFunc wasiFunc,
	paramTypes []api.ValueType,
	paramNames ...string,
) *wasm.HostFunc {
	return &wasm.HostFunc{
		ExportName:  name,
		Name:        name,
		ParamTypes:  paramTypes,
		ParamNames:  paramNames,
		ResultTypes: []api.ValueType{i32},
		ResultNames: []string{"errno"},
		Code:        wasm.Code{GoFunc: goFunc},
	}
}

func newHostMethod(
	name string,
	goFunc wasiMethod,
	paramTypes []api.ValueType,
	paramNames ...string,
) *wasm.HostFunc {
	return &wasm.HostFunc{
		ExportName:  name,
		Name:        name,
		ParamTypes:  paramTypes,
		ParamNames:  paramNames,
		ResultTypes: []api.ValueType{},
		ResultNames: []string{},
		Code:        wasm.Code{GoFunc: goFunc},
	}
}

// wasiFunc special cases that all WASI functions return a single Errno
// result. The returned value will be written back to the stack at index zero.
type wasiFunc func(ctx context.Context, mod api.Module, params []uint64) int32

// Call implements the same method as documented on api.GoModuleFunction.
func (f wasiFunc) Call(ctx context.Context, mod api.Module, stack []uint64) {
	// Write the result back onto the stack
	errno := f(ctx, mod, stack)
	if errno != 0 {
		stack[0] = uint64(errno)
	} else { // special case ass ErrnoSuccess is zero
		stack[0] = 0
	}
}

type wasiMethod func(ctx context.Context, mod api.Module, params []uint64)

func (f wasiMethod) Call(ctx context.Context, mod api.Module, stack []uint64) {
	// Write the result back onto the stack
	f(ctx, mod, stack)
}

// stubFunction stubs for GrainLang per #271.
func stubFunction(name string, paramTypes []wasm.ValueType, paramNames ...string) *wasm.HostFunc {
	return &wasm.HostFunc{
		ExportName:  name,
		Name:        name,
		ParamTypes:  paramTypes,
		ParamNames:  paramNames,
		ResultTypes: []api.ValueType{i32},
		ResultNames: []string{"errno"},
		Code: wasm.Code{
			GoFunc: api.GoModuleFunc(func(_ context.Context, _ api.Module, stack []uint64) { stack[0] = uint64(wasip1.ErrnoNosys) }),
		},
	}
}
