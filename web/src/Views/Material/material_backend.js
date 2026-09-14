import axios from "axios";

class MaterialBackend {
	getAllMaterials(formData, stoken) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/material/getall`,
					formData,
					{
						headers: {
							"SESSION-TOKEN": stoken,
						},
					}
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}

	addNewMaterial(formData, stoken) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/material/add`,
					formData,
					{
						headers: {
							"SESSION-TOKEN": stoken,
						},
					}
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}

	editMaterial(formData, stoken) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/material/update`,
					formData,
					{
						headers: {
							"SESSION-TOKEN": stoken,
						},
					}
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}

	deleteMaterial(formData, stoken) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/material/remove`,
					formData,
					{
						headers: {
							"SESSION-TOKEN": stoken,
						},
					}
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
}

export let materialsBackend = new MaterialBackend();
