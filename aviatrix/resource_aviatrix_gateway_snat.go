package aviatrix

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/terraform-providers/terraform-provider-aviatrix/goaviatrix"
)

func resourceAviatrixGatewaySNat() *schema.Resource {
	return &schema.Resource{
		Create: resourceAviatrixGatewaySNatCreate,
		Read:   resourceAviatrixGatewaySNatRead,
		Update: resourceAviatrixGatewaySNatUpdate,
		Delete: resourceAviatrixGatewaySNatDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"gw_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the gateway.",
			},
			"snat_mode": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "single_ip",
				Description: "Valid values: 'single_ip', and 'customized_snat'.",
			},
			"snat_policy": {
				Type:        schema.TypeList,
				Optional:    true,
				Default:     nil,
				Description: "Policy rule applied for 'snat_mode'' of 'custom'.'",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"src_cidr": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "A source IP address range where the policy rule applies.",
						},
						"src_port": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "A source port where the policy rule applies.",
						},
						"dst_cidr": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "A destination IP address range where the policy rule applies.",
						},
						"dst_port": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "A destination port where the policy rule applies.",
						},
						"protocol": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "A destination port protocol where the policy rule applies.",
						},
						"interface": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "An output interface where the policy rule applies.",
						},
						"connection": {
							Type:        schema.TypeString,
							Optional:    true,
							Default:     "None",
							Description: "None.",
						},
						"mark": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "A tag or mark of a TCP session where the policy rule applies.",
						},
						"snat_ips": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "The changed source IP address when all specified qualifier conditions meet. One of the rule fields must be specified for this rule to take effect.",
						},
						"snat_port": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "The translated destination port when all specified qualifier conditions meet. One of the rule field must be specified for this rule to take effect.",
						},
						"exclude_rtb": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "This field specifies which VPC private route table will not be programmed with the default route entry.",
						},
					},
				},
			},
		},
	}
}

func resourceAviatrixGatewaySNatCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*goaviatrix.Client)

	gateway := &goaviatrix.Gateway{
		GatewayName: d.Get("gw_name").(string),
	}

	snatMode := d.Get("snat_mode").(string)
	if snatMode == "single_ip" {
		gateway.EnableNat = "yes"
		if len(d.Get("snat_policy").([]interface{})) != 0 {
			return fmt.Errorf("'snat_policy' should be empty for 'snat_mode' of 'single_ip'")
		}
	} else if snatMode == "multiple_ips " {
		if len(d.Get("snat_policy").([]interface{})) != 0 {
			return fmt.Errorf("'snat_policy' should be empty for 'snat_mode' of 'multiple_ips'")
		}
		gateway.EnableNat = "yes"
		gateway.SnatMode = "secondary"
	} else if snatMode == "customized_snat" {
		if len(d.Get("snat_policy").([]interface{})) == 0 {
			return fmt.Errorf("please specify 'snat_policy' for 'snat_mode' of 'customized_snat'")
		}
		gateway.EnableNat = "yes"
		gateway.SnatMode = "custom"
		if _, ok := d.GetOk("snat_policy"); ok {
			policies := d.Get("snat_policy").([]interface{})
			for _, policy := range policies {
				pl := policy.(map[string]interface{})
				customPolicy := &goaviatrix.PolicyRule{
					SrcIP:      pl["src_cidr"].(string),
					SrcPort:    pl["src_port"].(string),
					DstIP:      pl["dst_cidr"].(string),
					DstPort:    pl["dst_port"].(string),
					Protocol:   pl["protocol"].(string),
					Interface:  pl["interface"].(string),
					Connection: pl["connection"].(string),
					Mark:       pl["mark"].(string),
					NewSrcIP:   pl["snat_ips"].(string),
					NewSrcPort: pl["snat_port"].(string),
					ExcludeRTB: pl["exclude_rtb"].(string),
				}
				gateway.SnatPolicy = append(gateway.SnatPolicy, *customPolicy)
			}
		}
	} else {
		return fmt.Errorf("please specify valid value for 'snat_mode'('single_ip' or 'customized_snat')")
	}
	err := client.EnableSNat(gateway)
	if err != nil {
		return fmt.Errorf("failed to enable SNAT of mode: %s for gateway(name: %s) due to: %s", snatMode, gateway.GatewayName, err)
	}

	d.SetId(gateway.GatewayName)
	return resourceAviatrixGatewaySNatRead(d, meta)
}

func resourceAviatrixGatewaySNatRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*goaviatrix.Client)

	gwName := d.Get("gw_name").(string)
	if gwName == "" {
		id := d.Id()
		log.Printf("[DEBUG] Looks like an import, no gateway name received. Import Id is %s", id)
		d.Set("gw_name", id)
		d.SetId(id)
	}

	gateway := &goaviatrix.Gateway{
		GwName: d.Get("gw_name").(string),
	}

	gw, err := client.GetGateway(gateway)
	if err != nil {
		if err == goaviatrix.ErrNotFound {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("couldn't find Aviatrix gateway: %s", err)
	}

	log.Printf("[TRACE] reading gateway %s: %#v", d.Get("gw_name").(string), gw)
	if gw != nil {
		d.Set("gw_name", gw.GwName)

		gwDetail, err := client.GetGatewayDetail(gateway)
		if err != nil {
			return fmt.Errorf("couldn't get detail information of Aviatrix gateway(name: %s) due to: %s", gw.GwName, err)
		}
		if gw.EnableNat == "yes" {
			if gw.SnatMode == "customized" {
				d.Set("snat_mode", "customized_snat")
				var snatPolicy []map[string]interface{}
				for _, policy := range gwDetail.SnatPolicy {
					sP := make(map[string]interface{})
					sP["src_cidr"] = policy.SrcIP
					sP["src_port"] = policy.SrcPort
					sP["dst_cidr"] = policy.DstIP
					sP["dst_port"] = policy.DstPort
					sP["protocol"] = policy.Protocol
					sP["interface"] = policy.Interface
					sP["connection"] = policy.Connection
					sP["mark"] = policy.Mark
					sP["snat_ips"] = policy.NewSrcIP
					sP["snat_port"] = policy.NewSrcPort
					sP["exclude_rtb"] = policy.ExcludeRTB
					snatPolicy = append(snatPolicy, sP)
				}

				if err := d.Set("snat_policy", snatPolicy); err != nil {
					log.Printf("[WARN] Error setting 'snat_policy' for (%s): %s", d.Id(), err)
				}
			} else if gw.SnatMode == "secondary" {
				d.Set("snat_mode", "multiple_ips")
				d.Set("snat_policy", nil)
			} else {
				d.Set("snat_mode", "single_ip")
				d.Set("snat_policy", nil)
			}
		} else {
			return fmt.Errorf("snat is not enabled for Aviatrix gateway: %s", gw.GwName)
		}
	}

	return nil
}

func resourceAviatrixGatewaySNatUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*goaviatrix.Client)

	log.Printf("[INFO] Updating Aviatrix gateway: %#v", d.Get("gw_name").(string))

	d.Partial(true)
	gateway := &goaviatrix.Gateway{
		GatewayName: d.Get("gw_name").(string),
	}

	if d.HasChange("snat_mode") {
		snatMode := d.Get("snat_mode").(string)
		if snatMode == "multiple_ips" {
			if len(d.Get("snat_policy").([]interface{})) != 0 {
				return fmt.Errorf("'snat_policy' should be empty for 'snat_mode' of 'multiple_ips'")
			}
			gateway.SnatMode = "secondary"
		} else if snatMode == "customized_snat" {
			if len(d.Get("snat_policy").([]interface{})) == 0 {
				return fmt.Errorf("please specify 'snat_policy' for 'snat_mode' of 'customized_snat'")
			}
			gateway.SnatMode = "custom"
			if _, ok := d.GetOk("snat_policy"); ok {
				policies := d.Get("snat_policy").([]interface{})
				for _, policy := range policies {
					pl := policy.(map[string]interface{})
					customPolicy := &goaviatrix.PolicyRule{
						SrcIP:      pl["src_cidr"].(string),
						SrcPort:    pl["src_port"].(string),
						DstIP:      pl["dst_cidr"].(string),
						DstPort:    pl["dst_port"].(string),
						Protocol:   pl["protocol"].(string),
						Interface:  pl["interface"].(string),
						Connection: pl["connection"].(string),
						Mark:       pl["mark"].(string),
						NewSrcIP:   pl["snat_ips"].(string),
						NewSrcPort: pl["snat_port"].(string),
						ExcludeRTB: pl["exclude_rtb"].(string),
					}
					gateway.SnatPolicy = append(gateway.SnatPolicy, *customPolicy)
				}
			}
		} else if snatMode == "single_ip" {
			if len(d.Get("snat_policy").([]interface{})) != 0 {
				return fmt.Errorf("'snat_policy' should be empty for 'snat_mode' of 'single_ip'")
			}
		} else {
			return fmt.Errorf("please specify valid value for 'snat_mode'('single_ip', or 'customized_snat')")
		}
		err := client.DisableSNat(gateway)
		if err != nil {
			return fmt.Errorf("failed to disable SNAT for gateway(name: %s) due to: %s", gateway.GatewayName, err)
		}
		err = client.EnableSNat(gateway)
		if err != nil {
			return fmt.Errorf("failed to enable SNAT of 'single_ip' for gateway(name: %s) due to: %s", gateway.GatewayName, err)
		}
	}

	if d.HasChange("snat_policy") {
		if !d.HasChange("snat_mode") {
			snatMode := d.Get("snat_mode").(string)
			if snatMode != "customized_snat" {
				return fmt.Errorf("cann't update 'snat_policy' for 'snat_mode': %s", snatMode)
			}
			if len(d.Get("snat_policy").([]interface{})) == 0 {
				return fmt.Errorf("please specify 'snat_policy' for 'snat_mode' of 'customized_snat'")
			}

			gateway.SnatMode = "custom"
			if _, ok := d.GetOk("snat_policy"); ok {
				policies := d.Get("snat_policy").([]interface{})
				for _, policy := range policies {
					pl := policy.(map[string]interface{})
					customPolicy := &goaviatrix.PolicyRule{
						SrcIP:      pl["src_cidr"].(string),
						SrcPort:    pl["src_port"].(string),
						DstIP:      pl["dst_cidr"].(string),
						DstPort:    pl["dst_port"].(string),
						Protocol:   pl["protocol"].(string),
						Interface:  pl["interface"].(string),
						Connection: pl["connection"].(string),
						Mark:       pl["mark"].(string),
						NewSrcIP:   pl["snat_ips"].(string),
						NewSrcPort: pl["snat_port"].(string),
						ExcludeRTB: pl["exclude_rtb"].(string),
					}
					gateway.SnatPolicy = append(gateway.SnatPolicy, *customPolicy)
				}
			}

			err := client.EnableSNat(gateway)
			if err != nil {
				return fmt.Errorf("failed to enable SNAT of 'customized_snat': %s", err)
			}
		}
	}

	d.Partial(false)
	d.SetId(d.Get("gw_name").(string))
	return resourceAviatrixGatewaySNatRead(d, meta)
}

func resourceAviatrixGatewaySNatDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*goaviatrix.Client)
	gateway := &goaviatrix.Gateway{
		GatewayName: d.Get("gw_name").(string),
	}

	err := client.DisableSNat(gateway)
	if err != nil {
		return fmt.Errorf("failed to disable SNAT for Aviatrix gateway(name: %s) due to: %s", gateway.GatewayName, err)
	}

	return nil
}
