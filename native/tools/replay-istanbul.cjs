// SPDX-License-Identifier: EPL-2.0
// Developer/validation producer only. Not embedded in the native renderer or release.
'use strict';
const fs=require('node:fs');const path=require('node:path');const crypto=require('node:crypto');const Module=require('node:module');
const {createInstrumenter}=require('istanbul-lib-instrument');const {createCoverageMap}=require('istanbul-lib-coverage');const {createSourceMapStore}=require('istanbul-lib-source-maps');const ts=require('typescript');
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
function inside(root,p){const r=path.relative(root,p);return r!== '..'&&!r.startsWith('..'+path.sep)&&!path.isAbsolute(r)}
function local(root,p){p=path.resolve(p);if(!inside(root,p)||fs.realpathSync(p)!==p||!fs.lstatSync(p).isFile())throw Error('unsafe source '+p);return p}
async function main(){const [kind,input,corpus,out]=process.argv.slice(2);if(!['js','ts'].includes(kind)||!out)throw Error('usage: js|ts TARGET CORPUS NEW_OUTPUT');const target=fs.realpathSync(input),root=path.dirname(target),output=path.resolve(out);local(root,target);if(inside(root,output)||inside(output,root))throw Error('output/source overlap');fs.mkdirSync(output);const original=fs.readFileSync(target);let code=original.toString('utf8'),generatedPath=target,map;
 if(kind==='ts'){
  // Controlled compiler input; sources are explicitly reset to the exact original
  // absolute file, eliminating automatic URI/sourceRoot reads by MapStore.
  const compiled=ts.transpileModule(code,{fileName:target,compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022,sourceMap:true,inlineSources:true}});
  code=compiled.outputText;map=JSON.parse(compiled.sourceMapText);if(map.sources.length!==1||map.sourcesContent.length!==1||map.sourcesContent[0]!==original.toString('utf8'))throw Error('compiler map mismatch');
  fs.writeFileSync(path.join(output,'compiler.map.json'),compiled.sourceMapText);
  map.sourceRoot='';map.sources=[target];generatedPath=path.join(output,'generated.js');map.file=generatedPath;
  fs.writeFileSync(path.join(output,'resolved.map.json'),JSON.stringify(map));fs.writeFileSync(generatedPath,code);
 }
 const instrumenter=createInstrumenter({esModules:false,produceSourceMap:true});const instrumented=instrumenter.instrumentSync(code,generatedPath,map);
 const mod=new Module(generatedPath,module);mod.filename=generatedPath;mod.paths=Module._nodeModulePaths(path.dirname(generatedPath));mod._compile(instrumented,generatedPath);
 const records=[];for(const name of fs.readdirSync(corpus).sort()){const p=path.resolve(corpus,name);if(fs.lstatSync(p).isSymbolicLink()||!fs.lstatSync(p).isFile())throw Error('corpus must contain regular files');const bytes=fs.readFileSync(p);await mod.exports.fuzz(bytes);records.push({name,sha256:hash(bytes)});}
 const raw=JSON.parse(JSON.stringify(global.__coverage__));fs.writeFileSync(path.join(output,'istanbul-generated.json'),JSON.stringify(raw,null,2));
 const store=createSourceMapStore();if(map)store.registerMap(generatedPath,map);const remapped=await store.transformCoverage(createCoverageMap(raw));const result=remapped.toJSON();
 if(Object.keys(result).length!==1||!result[target])throw Error('expected exact original target coverage');for(const file of Object.keys(result)){local(root,file);if(store.sourceFinder(file)!==fs.readFileSync(file,'utf8'))throw Error('source map content mismatch');}
 fs.writeFileSync(path.join(output,'istanbul.json'),JSON.stringify(result,null,2));fs.writeFileSync(path.join(output,'upstream-summary.json'),JSON.stringify(remapped.getCoverageSummary().toJSON(),null,2));
 fs.writeFileSync(path.join(output,'provenance.json'),JSON.stringify({execution_owned_by:'external',producer:'AST Istanbul',node:process.version,v8:process.versions.v8,tools:Object.fromEntries(['istanbul-lib-instrument','istanbul-lib-coverage','istanbul-lib-source-maps','typescript'].map(n=>[n,require(n+'/package.json').version])),target:{path:target,sha256:hash(original)},generated:map?{path:generatedPath,sha256:hash(Buffer.from(code)),compiler_map_sha256:hash(fs.readFileSync(path.join(output,'compiler.map.json'))),resolved_map_sha256:hash(fs.readFileSync(path.join(output,'resolved.map.json')))}:null,corpus:records,command:process.argv,replay_process_status:'completed-observed-by-producer'},null,2));
}
main().catch(e=>{console.error(e);process.exitCode=1});
