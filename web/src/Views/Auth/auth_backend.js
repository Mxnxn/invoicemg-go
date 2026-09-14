import axios from "axios";

class AuthBackend {
	loginWithEmailAndPassword(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/user/login`,
					formData
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
	loginAsEmployee(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/person/login`,
					formData
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
	registerWithEmailAndPassword(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/user/register`,
					formData
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}

	requestPasswordReset(email) {
		return new Promise(async (resolve, reject) => {
			try {
				const formData = new FormData();
				formData.set("email", email);
				const res = await axios.post(`${import.meta.env.VITE_API_URL}/user/password/request-reset`, formData);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
}
export let authBackend = new AuthBackend();
