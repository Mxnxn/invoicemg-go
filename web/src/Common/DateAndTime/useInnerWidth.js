import { useEffect, useState } from "react";

const useInnerWidth = () => {
    const [innerW, setInnerW] = useState(window.innerWidth);

    useEffect(() => {
        const updateWindowDimensions = () => {
            const newW = window.innerWidth;
            setInnerW(newW);
        };

        window.addEventListener("resize", updateWindowDimensions);

        return () => window.removeEventListener("resize", updateWindowDimensions);
    }, []);

    return innerW;
};

export default useInnerWidth;
