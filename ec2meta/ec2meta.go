package ec2meta

import (
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"
)

// Instance contains all aws instance metadata.
type Instance struct {
	AmiID            string `json:"ami-id"`
	AvailabilityZone string `json:"availability-zone"`
	Hostname         string `json:"hostname"`
	InstanceID       string `json:"instance-id"`
	InstanceType     string `json:"instance-type"`
	LocalIpv4        string `json:"local-ipv4"`
	LocalHostname    string `json:"local-hostname"`
	PublicHostname   string `json:"public-hostname"`
	PublicIpv4       string `json:"public-ipv4"`
	ReservationID    string `json:"reservation-id"`
	SecurityGroups   string `json:"security-groups"`
	Identity         struct {
		Document time.Time `json:"document"`
		Pkcs7    string    `json:"pkcs7"`
	} `json:"identity"`
	Account     string `json:"account"`
	Region      string `json:"region"`
	IamRole     string `json:"iam-role"`
	Credentials struct {
		LastUpdated     time.Time `json:"lastUpdated"`
		Type            string    `json:"type"`
		AccessKeyID     string    `json:"accessKeyId"`
		SecretAccessKey string    `json:"secretAccessKey"`
		Expires         time.Time `json:"expires"`
	} `json:"credentials"`
	Interface struct {
		VpcIpv4CidrBlock    string `json:"vpc-ipv4-cidr-block"`
		SubnetIpv4CidrBlock string `json:"subnet-ipv4-cidr-block"`
		PublicIpv4S         string `json:"public-ipv4s"`
		Mac                 string `json:"mac"`
		LocalIpv4S          string `json:"local-ipv4s"`
		InterfaceID         string `json:"interface-id"`
	} `json:"interface"`
	VpcID            string   `json:"vpc-id"`
	AutoScalingGroup string   `json:"auto-scaling-group"`
	Tags             struct{} `json:"tags"`
}

// Populate populates Instance with real or mock data depending
// on the environment.
func Populate(sess *session.Session, log *zap.Logger) Instance {
	svc := ec2metadata.New(sess)

	if _, err := os.Stat("/usr/bin/ec2metadata"); err != nil {

		log.Error("Could not find local ec2metadata binary", zap.Error(err))
		metadata := Instance{
			AmiID:            "ami-bcbffad6",
			AvailabilityZone: "us-east-1a",
			Hostname:         "ip-10-196-24-63.ec2.internal",
			InstanceID:       "i-aaaf2d1a",
			InstanceType:     "t2.small",
			LocalIpv4:        "10.196.24.63",
			LocalHostname:    "ip-10-196-24-63.ec2.internal",
			PublicHostname:   "ec2-1-2-3-4.compute-1.amazonaws.com",
			PublicIpv4:       "1.2.3.4",
			ReservationID:    "r-fake",
			SecurityGroups:   "fake-fake\nfoo-bar-baz",
			Account:          "000000000000",
			Region:           "us-east-1",
			VpcID:            "vpc-fake",
		}

		return metadata
	}

	metadata := Instance{
		AmiID:            getAmiID(svc, log),
		AvailabilityZone: getAvailabilityZone(svc, log),
		Hostname:         getHostname(svc, log),
		InstanceID:       getInstanceID(svc, log),
		InstanceType:     getInstanceType(svc, log),
		LocalIpv4:        getLocalIpv4(svc, log),
		LocalHostname:    getLocalHostname(svc, log),
		PublicHostname:   getPublicHostname(svc, log),
		PublicIpv4:       getPublicIpv4(svc, log),
		ReservationID:    getReservationID(svc, log),
		SecurityGroups:   getSecurityGroups(svc, log),
		Account:          getAccount(svc, log),
		Region:           getRegion(svc, log),
		VpcID:            getVpcID(svc, log),
	}

	return metadata
}

func getAmiID(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	id, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		log.Error("could not get ami id", zap.Error(err))
		return ""
	}

	return id.ImageID
}

func getAvailabilityZone(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	id, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		log.Error("could not get availability zone", zap.Error(err))
		return ""
	}

	return id.AvailabilityZone
}

func getHostname(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	h, err := svc.GetMetadata("/hostname")
	if err != nil {
		log.Error("could not get hostname", zap.Error(err))
		return ""
	}

	return h
}

func getInstanceID(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	id, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		log.Error("could not get instance id", zap.Error(err))
		return ""
	}

	return id.InstanceID
}

func getInstanceType(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	id, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		log.Error("could not get instance type", zap.Error(err))
		return ""
	}

	return id.InstanceType
}

func getLocalIpv4(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	i, err := svc.GetMetadata("/local-ipv4")
	if err != nil {
		log.Error("could not get local ipv4", zap.Error(err))
		return ""
	}

	return i
}

func getLocalHostname(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	h, err := svc.GetMetadata("/local-hostname")
	if err != nil {
		log.Error("could not get local hostname", zap.Error(err))
		return ""
	}

	return h
}

func getPublicHostname(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	h, err := svc.GetMetadata("/public-hostname")
	if err != nil {
		log.Error("could not get public hostname", zap.Error(err))
		return ""
	}

	return h
}

func getPublicIpv4(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	i, err := svc.GetMetadata("/public-ipv4")
	if err != nil {
		log.Error("could not get public ipv4", zap.Error(err))
		return ""
	}

	return i
}

func getReservationID(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	r, err := svc.GetMetadata("/reservation-id")
	if err != nil {
		log.Error("could not get reservation id", zap.Error(err))
		return ""
	}

	return r
}

func getSecurityGroups(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	s, err := svc.GetMetadata("/security-groups")
	if err != nil {
		log.Error("could not get security groups", zap.Error(err))
		return ""
	}

	return s
}

func getAccount(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	id, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		log.Error("could not get account id", zap.Error(err))
		return ""
	}

	return id.AccountID
}

func getRegion(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	id, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		log.Error("could not get region", zap.Error(err))
		return ""
	}

	return id.Region
}

func getVpcID(svc *ec2metadata.EC2Metadata, log *zap.Logger) string {
	m, err := svc.GetMetadata("/network/interfaces/macs/")
	if err != nil {
		log.Error("could not get interface mac addresses", zap.Error(err))
		return ""
	}

	firstMac := strings.Split(m, "\n")[0]

	v, err := svc.GetMetadata("/network/interfaces/macs/" + firstMac + "/vpc-id")
	if err != nil {
		log.Error("could not get vpc id", zap.Error(err))
		return ""
	}

	return v
}
