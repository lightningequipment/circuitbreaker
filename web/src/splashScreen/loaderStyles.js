const loaderStyles = `
#globalLoader {
	display: flex;
	position: fixed;
	z-index: 10000;
	top: 0;
	left: 0;
	right: 0;
	bottom: 0;
	justify-content: center;
	align-items: center;
	background-color: background: linear-gradient(0deg, #0e101b -19.82%, #060712 64.16%), #ffffff;
	transition: opacity 250ms ease-in-out 0ms;
	opacity: 1;
}
`;

export default loaderStyles;
