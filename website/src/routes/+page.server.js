export const load = async () => {
	const fetchJSON = async () => {
		const res = await fetch(`http://` + import.meta.env.VITE_DATA_URL + `/cache.json`);
		const data = await res.json();
		return data;
	};

	return fetchJSON();
};
