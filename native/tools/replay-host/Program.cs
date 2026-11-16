// SPDX-License-Identifier: EPL-2.0
using System.Reflection;
using System.Runtime.Loader;
using System.Security.Cryptography;
using System.Text.Json;
if(args.Length!=5)throw new ArgumentException("ASSEMBLY CORPUS LOG TYPE METHOD");
string assemblyPath=Path.GetFullPath(args[0]);
Assembly assembly=AssemblyLoadContext.Default.LoadFromAssemblyPath(assemblyPath);
var method=assembly.GetType(args[3],throwOnError:true)!.GetMethod(args[4],BindingFlags.Public|BindingFlags.Static,null,new[]{typeof(byte[])},null) ?? throw new MissingMethodException("Expected public static method(byte[])");
if(method.ReturnType!=typeof(void))throw new ArgumentException("Replay requires a void method(byte[])");
// Read loaded on-disk bytes while Coverlet owns instrumentation, before restore.
var loaded=new{location=assembly.Location,sha256=Convert.ToHexStringLower(SHA256.HashData(File.ReadAllBytes(assembly.Location)))};
var inputs=new List<object>();
foreach(string input in Directory.EnumerateFiles(args[1]).Order(StringComparer.Ordinal)){
 if((File.GetAttributes(input)&FileAttributes.ReparsePoint)!=0)throw new IOException("symlink corpus");
 byte[] bytes=File.ReadAllBytes(input);method.Invoke(null,new object[]{bytes});
 inputs.Add(new{name=Path.GetFileName(input),sha256=Convert.ToHexStringLower(SHA256.HashData(bytes)),status="returned"});
}
File.WriteAllText(args[2],JsonSerializer.Serialize(new{loaded,inputs,observed_exit_code=0},new JsonSerializerOptions{WriteIndented=true}));
