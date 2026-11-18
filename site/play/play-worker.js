importScripts("wasm_exec.js");

async function start() {
  const go = new Go();
  const response = await fetch("dinoscript.wasm");
  if (!response.ok) throw new Error(`Wasm download failed (${response.status})`);
  const isWasm = response.headers.get("content-type")?.split(";")[0] === "application/wasm";
  const { instance } = isWasm && WebAssembly.instantiateStreaming
    ? await WebAssembly.instantiateStreaming(response, go.importObject)
    : await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
  await go.run(instance);
  throw new Error("The Dinoscript worker exited");
}

start().catch(error => {
  console.error(error);
  postMessage({ kind: "error" });
});
