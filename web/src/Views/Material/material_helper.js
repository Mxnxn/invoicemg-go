import Material from "./material_model";

export default class MaterialHelper {
  toMaterialEntry(id, data) {
    return new Material(id, data.material_name, data.material_rate, data.purchase_rate, data.hsn, data.tax, data.priceHistory, data.unit);
  }
}
