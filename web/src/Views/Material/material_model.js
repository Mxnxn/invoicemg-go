export default class Material {
  constructor(id, material_name, material_rate, purchase_rate, hsn, tax, priceHistory, unit) {
    this.id = id;
    this.material_name = material_name;
    this.material_rate = material_rate;
    this.purchase_rate = purchase_rate;
    this.hsn = hsn;
    this.tax = tax;
    this.priceHistory = priceHistory || [];
    // Stocked unit - drives the inventory report's consumption rule.
    this.unit = unit || "";
  }

  getObject() {
    return {
      id: this.id,
      material_name: this.material_name,
      material_rate: this.material_rate,
      purchase_rate: this.purchase_rate,
      hsn: this.hsn,
      tax: this.tax,
      priceHistory: this.priceHistory,
      unit: this.unit,
    };
  }
}
