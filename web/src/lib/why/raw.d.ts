// Local stand-in for the `vite/client` `*?raw` declaration: the web
// tsconfig carries no vite client types, and adding them sits outside this
// task's paths, so the one raw import the simplicity guards need is typed
// here, beside its only use.
declare module '*?raw' {
	const content: string;
	export default content;
}
